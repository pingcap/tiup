// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package proc

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/utils"
)

// ServiceID is a logical service identifier in playground.
//
// A ServiceID represents one start behavior (args/config/ports/ready strategy),
// and is what playground orchestration should use for planning, dependency and
// "critical" semantics.
//
// ServiceID is mode-independent. Mode only affects how a ServiceID maps to a
// RepoComponentID and its start implementation.
type ServiceID string

func (id ServiceID) String() string { return string(id) }

// RepoComponentID is a TiUP repository component ID.
//
// It is used for version resolution, downloading and locating binaries.
type RepoComponentID string

func (id RepoComponentID) String() string { return string(id) }

var componentDisplayNames map[RepoComponentID]string
var serviceDisplayNames map[ServiceID]string

// RegisterComponentDisplayName registers a user-facing name for a repository
// component ID.
//
// It is intended to be called from the init() function of each component
// implementation, so the naming rules stay close to the component itself.
func RegisterComponentDisplayName(componentID RepoComponentID, displayName string) {
	if componentID == "" || displayName == "" {
		return
	}
	if componentDisplayNames == nil {
		componentDisplayNames = make(map[RepoComponentID]string)
	}
	componentDisplayNames[componentID] = displayName
}

// ComponentDisplayName returns a user-facing name for a repository component ID.
//
// If no display name is registered, it falls back to a best-effort title-cased
// version of the id (split by '-' or '_').
func ComponentDisplayName(componentID RepoComponentID) string {
	if componentID == "" {
		return ""
	}
	if componentDisplayNames != nil {
		if s := componentDisplayNames[componentID]; s != "" {
			return s
		}
	}
	return titleCaseComponentID(componentID.String())
}

// RegisterServiceDisplayName registers a user-facing name for a service.
//
// It is intended to be called from the init() function of each service
// implementation, so the naming rules stay close to the component itself.
func RegisterServiceDisplayName(serviceID ServiceID, displayName string) {
	if serviceID == "" || displayName == "" {
		return
	}
	if serviceDisplayNames == nil {
		serviceDisplayNames = make(map[ServiceID]string)
	}
	serviceDisplayNames[serviceID] = displayName
}

// ServiceDisplayName returns a user-facing name for a service.
//
// If no service-specific display name is registered, it falls back to a
// best-effort title-cased form of the service ID.
func ServiceDisplayName(serviceID ServiceID) string {
	if serviceID == "" {
		return ""
	}
	if serviceDisplayNames != nil {
		if s := serviceDisplayNames[serviceID]; s != "" {
			return s
		}
	}
	return titleCaseComponentID(string(serviceID))
}

func titleCaseComponentID(id string) string {
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '-' || r == '_'
	})
	if len(parts) == 0 {
		return id
	}
	for i, p := range parts {
		parts[i] = capitalizeASCII(p)
	}
	return strings.Join(parts, " ")
}

func capitalizeASCII(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] = b[0] - 'a' + 'A'
	}
	return string(b)
}

// Mode of playground
type Mode = string

var (
	// ModeNormal is the default mode.
	ModeNormal = "tidb"
	// ModeCSE is for CSE testing.
	ModeCSE = "tidb-cse"
	// ModeNextGen is for NG testing.
	ModeNextGen = "tidb-x"
	// ModeDisAgg is for tiflash testing.
	ModeDisAgg = "tiflash-disagg"
	// ModeTiKVSlim is for special tikv testing.
	ModeTiKVSlim = "tikv-slim"
)

// Config of the instance.
type Config struct {
	ConfigPath string `yaml:"config_path"`
	BinPath    string `yaml:"bin_path"`
	Num        int    `yaml:"num"`
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	UpTimeout  int    `yaml:"up_timeout"`
	Version    string `yaml:"version"`
}

// SharedOptions contains some commonly used, tunable options for most components.
// Unlike Config, these options are shared for all instances of all components.
type SharedOptions struct {
	/// Whether or not to tune the cluster in order to run faster (instead of easier to debug).
	HighPerf           bool       `yaml:"high_perf"`
	CSE                CSEOptions `yaml:"cse"` // Only available when mode == ModeCSE or ModeDisAgg
	PDMode             string     `yaml:"pd_mode"`
	Mode               string     `yaml:"mode"`
	PortOffset         int        `yaml:"port_offset"`
	EnableTiKVColumnar bool       `yaml:"enable_tikv_columnar"` // Only available when mode == ModeCSE
	ForcePull          bool       `yaml:"force_pull"`
}

// CSEOptions contains configs to run TiDB cluster in CSE mode.
type CSEOptions struct {
	S3Endpoint string `yaml:"s3_endpoint"`
	Bucket     string `yaml:"bucket"`
	AccessKey  string `yaml:"access_key"`
	SecretKey  string `yaml:"secret_key"`
}

var (
	errNotUp = errors.New("not up")
)

// OSProcess represents an operating system process started by playground.
type OSProcess interface {
	Start() error
	Wait() error
	Pid() int
	Uptime() string
	SetOutputFile(fname string) error
	Cmd() *exec.Cmd
}

// cmdProcess implements OSProcess via an *exec.Cmd.
type cmdProcess struct {
	cmd       *exec.Cmd
	startTime time.Time
	endTime   time.Time

	waitOnce sync.Once
	waitErr  error

	outFile *os.File
}

// Start the process
func (p *cmdProcess) Start() error {
	if p == nil {
		return errNotUp
	}

	// fmt.Printf("Starting `%s`: %s", filepath.Base(p.cmd.Path), strings.Join(p.cmd.Args, " "))
	p.startTime = time.Now()
	p.endTime = time.Time{}
	if err := p.cmd.Start(); err != nil {
		if p.outFile != nil {
			_ = p.outFile.Close()
			p.outFile = nil
		}
		return err
	}
	return nil
}

// Wait implements OSProcess.
func (p *cmdProcess) Wait() error {
	if p == nil {
		return errNotUp
	}

	p.waitOnce.Do(func() {
		p.waitErr = p.cmd.Wait()
		p.endTime = time.Now()
		if p.outFile != nil {
			_ = p.outFile.Close()
			p.outFile = nil
		}
	})

	return p.waitErr
}

// Pid implements OSProcess.
func (p *cmdProcess) Pid() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// Uptime implements OSProcess.
func (p *cmdProcess) Uptime() string {
	if p == nil {
		return "not started"
	}

	if p.startTime.IsZero() {
		return "not started"
	}

	end := p.endTime
	if end.IsZero() {
		end = time.Now()
	}
	duration := end.Sub(p.startTime)
	return duration.String()
}

func (p *cmdProcess) SetOutputFile(fname string) error {
	if p == nil {
		return errNotUp
	}

	if fname == "" {
		return errors.New("output file path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(fname), 0o755); err != nil {
		return errors.AddStack(err)
	}

	f, err := os.OpenFile(fname, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return errors.AddStack(err)
	}
	if p.outFile != nil {
		_ = p.outFile.Close()
	}
	p.outFile = f
	p.setOutput(f)
	return nil
}

func (p *cmdProcess) setOutput(w io.Writer) {
	if p == nil {
		return
	}

	p.cmd.Stdout = w
	p.cmd.Stderr = w
}

func (p *cmdProcess) Cmd() *exec.Cmd {
	if p == nil {
		return nil
	}

	return p.cmd
}

// PrepareCommand return command for playground
func PrepareCommand(ctx context.Context, binPath string, args, envs []string, workDir string) *exec.Cmd {
	c := exec.CommandContext(ctx, binPath, args...)

	c.Env = os.Environ()
	c.Env = append(c.Env, envs...)
	c.Dir = workDir
	c.SysProcAttr = SysProcAttr
	return c
}

// ProcessInfo holds the shared, low-level fields for a running playground
// instance.
//
// Concrete instances embed it and expose it via the Process.Info method.
type ProcessInfo struct {
	ID         int
	Dir        string
	Host       string
	Port       int
	StatusPort int // client port for PD
	// UpTimeout is the maximum wait time (in seconds) for the instance to become
	// ready.
	//
	// It is only used by components that implement readiness checks (e.g. TiDB,
	// TiProxy, TiFlash). A value <= 0 means no limit.
	UpTimeout  int
	ConfigPath string
	// UserBinPath is the binary path provided by the user (if any).
	//
	// It is treated as input and must not be overwritten by binary resolution.
	UserBinPath string
	// BinPath is the resolved executable path used to start the instance.
	//
	// It is set by the playground planner/preloader and may come from either a
	// user-provided path or a repository-installed component.
	BinPath         string
	Version         utils.Version
	Proc            OSProcess
	RepoComponentID RepoComponentID
	Service         ServiceID
}

// Info returns itself so embedded ProcessInfo can satisfy Process.
func (info *ProcessInfo) Info() *ProcessInfo { return info }

// MetricAddr will be used by prometheus scrape_configs.
type MetricAddr struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

// Process represent running component
type Process interface {
	Info() *ProcessInfo

	// Prepare builds the process command (config + args) for later start.
	//
	// It does NOT start the underlying process. Playground owns the actual
	// proc.Start() call so it can consistently wire logs, waiters and readiness.
	Prepare(ctx context.Context) error
	// LogFile return the log file name
	LogFile() string
}

// ReadyWaiter is an optional interface implemented by instances that need an
// explicit "ready" check after the process has been started.
//
// It is intentionally separated from the core Process interface so most
// components can remain "start-and-done", while components like TiDB / TiProxy /
// TiFlash can keep showing a spinner until they are actually ready to serve.
type ReadyWaiter interface {
	// WaitReady blocks until the instance is ready, or ctx is done.
	//
	// The maximum waiting time should usually be controlled by the instance's
	// UpTimeout field (in seconds). A value <= 0 means no limit.
	WaitReady(ctx context.Context) error
}

func readyTimeoutError(timeoutSec int) error {
	return fmt.Errorf("timeout (%ds)", timeoutSec)
}

func tcpAddrReady(ctx context.Context, addr string, timeoutSec int) error {
	if addr == "" {
		return fmt.Errorf("empty address")
	}

	ctx, cancel := withTimeoutSeconds(ctx, timeoutSec)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		// Use a short per-attempt timeout so overall timeout semantics match
		// timeoutSec, and so Ctrl+C can interrupt quickly.
		perAttempt := time.Second
		if deadline, ok := ctx.Deadline(); ok {
			remain := time.Until(deadline)
			if remain <= 0 {
				if timeoutSec > 0 {
					return readyTimeoutError(timeoutSec)
				}
				return context.DeadlineExceeded
			}
			if remain < perAttempt {
				perAttempt = remain
			}
		}

		conn, err := net.DialTimeout("tcp", addr, perAttempt)
		if err == nil {
			_ = conn.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			err := ctx.Err()
			if err == context.DeadlineExceeded && timeoutSec > 0 {
				return readyTimeoutError(timeoutSec)
			}
			return err
		case <-ticker.C:
		}
	}
}

func withTimeoutSeconds(ctx context.Context, timeoutSec int) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeoutSec > 0 {
		return context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	}
	return ctx, func() {}
}

func withLogger(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger); ok {
		return ctx
	}
	return context.WithValue(ctx, logprinter.ContextKeyLogger, logprinter.NewLogger(""))
}

// Name returns the stable instance name used by the underlying component flags
// and topology.
func (info *ProcessInfo) Name() string {
	if info == nil {
		return ""
	}

	prefix := info.Service.String()
	if prefix == "" {
		prefix = info.RepoComponentID.String()
	}
	if prefix == "" {
		prefix = "instance"
	}
	return fmt.Sprintf("%s-%d", prefix, info.ID)
}

// DisplayName returns the user-facing display name for an instance (no index
// included).
func (info *ProcessInfo) DisplayName() string {
	if info == nil {
		return ""
	}
	if info.Service != "" {
		if s := ServiceDisplayName(info.Service); s != "" {
			return s
		}
	}
	return ComponentDisplayName(info.RepoComponentID)
}

// MetricAddr returns the default address to pull metrics.
//
// Specific instances can override it by defining their own MetricAddr method.
func (info *ProcessInfo) MetricAddr() (r MetricAddr) {
	if info == nil {
		return r
	}
	if info.Host != "" && info.StatusPort != 0 {
		r.Targets = append(r.Targets, utils.JoinHostPort(info.Host, info.StatusPort))
	}
	return r
}

// AdvertiseHost returns the interface's ip addr if listen host is 0.0.0.0
func AdvertiseHost(listen string) string {
	if listen == "0.0.0.0" {
		addrs, err := net.InterfaceAddrs()
		if err != nil || len(addrs) == 0 {
			return "localhost"
		}

		for _, addr := range addrs {
			if ip, ok := addr.(*net.IPNet); ok && !ip.IP.IsLoopback() && ip.IP.To4() != nil {
				return ip.IP.To4().String()
			}
		}
		return "localhost"
	}

	return listen
}

func pdEndpoints(pds []*PDInstance, isHTTP bool) []string {
	var endpoints []string
	for _, pd := range pds {
		switch pd.Service {
		case ServicePDTSO, ServicePDScheduling, ServicePDRouter, ServicePDResourceManager:
			continue
		}
		if isHTTP {
			endpoints = append(endpoints, "http://"+utils.JoinHostPort(AdvertiseHost(pd.Host), pd.StatusPort))
		} else {
			endpoints = append(endpoints, utils.JoinHostPort(AdvertiseHost(pd.Host), pd.StatusPort))
		}
	}
	return endpoints
}

// prepareConfig writes a merged config to outputConfigPath.
//
// Merge order:
// 1) preDefinedConfig provides defaults
// 2) userConfigPath overrides defaults
// 3) forceOverride overwrites both (for runtime-required fields)
func prepareConfig(outputConfigPath string, userConfigPath string, preDefinedConfig map[string]any, forceOverride map[string]any) (err error) {
	dir := filepath.Dir(outputConfigPath)
	if err := utils.MkdirAll(dir, 0755); err != nil {
		return err
	}

	userConfig, err := unmarshalConfig(userConfigPath)
	if err != nil {
		return errors.Trace(err)
	}
	if userConfig == nil {
		userConfig = make(map[string]any)
	}

	cf, err := os.Create(outputConfigPath)
	if err != nil {
		return errors.Trace(err)
	}
	defer func() {
		cerr := cf.Close()
		if err == nil {
			err = cerr
		}
	}()

	enc := toml.NewEncoder(cf)
	enc.Indent = ""
	merged := spec.MergeConfig(preDefinedConfig, userConfig)
	if len(forceOverride) > 0 {
		merged = spec.MergeConfig(merged, forceOverride)
	}
	return enc.Encode(merged)
}

func unmarshalConfig(path string) (map[string]any, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := make(map[string]any)
	err = toml.Unmarshal(data, &c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func applyS3DFSConfig(config map[string]any, s3 CSEOptions, prefix string) {
	if config == nil {
		return
	}
	if prefix != "" {
		config["dfs.prefix"] = prefix
	}
	config["dfs.s3-endpoint"] = s3.S3Endpoint
	config["dfs.s3-key-id"] = s3.AccessKey
	config["dfs.s3-secret-key"] = s3.SecretKey
	config["dfs.s3-bucket"] = s3.Bucket
	config["dfs.s3-region"] = "local"
}
