package service

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/cluster/api"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/utils"
)

// Runtime is the minimal playground surface that service specs depend on.
//
// It is implemented by *main.Playground.
type Runtime interface {
	Booted() bool
	SharedOptions() proc.SharedOptions
	DataDir() string

	// BootConfig returns the boot-time base config for the given service.
	//
	// It is used to fill defaults for scale-out requests and to decide some
	// initial behaviors when creating instances.
	BootConfig(serviceID proc.ServiceID) (proc.Config, bool)

	Procs(serviceID proc.ServiceID) []proc.Process
	Stopping() bool
	EmitEvent(evt any)

	TermWriter() io.Writer
	OnProcsChanged()
}

// ControllerRuntime is the playground runtime surface only available to the
// controller goroutine.
//
// It extends Runtime with mutating operations over controller-owned state.
type ControllerRuntime interface {
	Runtime

	AddProc(serviceID proc.ServiceID, inst proc.Process)
	RemoveProc(serviceID proc.ServiceID, inst proc.Process) bool
	ExpectExitPID(pid int)
}

// Event is handled by playground's controller loop.
type Event interface {
	Handle(ControllerRuntime)
}

type NewProcParams struct {
	Config proc.Config
	ID     int
	Dir    string
	Host   string
}

type ScaleInHookFunc func(rt ControllerRuntime, w io.Writer, inst proc.Process, pid int) (async bool, err error)

type PostScaleOutFunc func(w io.Writer, inst proc.Process)

// BootContext is the minimal boot-time surface that service metadata depends on.
//
// It is implemented by *main.BootOptions.
//
// BootContext is guaranteed to be non-nil for all catalog callbacks
// (DefaultNum/IsEnabled/IsCritical/PlanConfig, etc). Callers must not pass nil.
type BootContext interface {
	SharedOptions() proc.SharedOptions
	BootVersion() string

	MonitorEnabled() bool
	GrafanaPortOverride() int

	// ServiceConfigFor returns the current config snapshot for a service.
	//
	// It is primarily used to express "default from another service" rules.
	ServiceConfigFor(serviceID proc.ServiceID) proc.Config
}

type VersionBindFunc func(baseVersion string) string

// Catalog is declarative metadata for a service.
//
// It is used to:
//   - register and interpret CLI flags (via `components/playground/catalog_flags.go`);
//   - compute default values derived from mode/other services;
//   - decide whether a service should be planned (booted) for a given BootContext;
//   - decide whether a service is "critical" (required) and whether it supports scale-out;
//   - customize version selection for repository component resolution.
//
// It is intentionally "data-first": most orchestration code should depend on
// this metadata rather than hardcoding service-specific special cases.
type Catalog struct {
	// FlagPrefix is the CLI flag namespace for this service.
	//
	// When non-empty, flags are registered under:
	//   - `--<FlagPrefix>`             (count, when HasCount is true)
	//   - `--<FlagPrefix>.host`        (host override, when HasHost is true)
	//   - `--<FlagPrefix>.port`        (port override, when HasPort is true)
	//   - `--<FlagPrefix>.config`      (config file path, when HasConfig is true)
	//   - `--<FlagPrefix>.binpath`     (binary path, when HasBinPath is true)
	//   - `--<FlagPrefix>.timeout`     (ready wait timeout seconds, when HasTimeout is true)
	//   - `--<FlagPrefix>.version`     (version constraint override, when HasVersion is true)
	//
	// When empty, this service is "internal-only" from the CLI perspective (it
	// may still be planned via EnabledWhen/PlanConfig).
	FlagPrefix string

	// AllowModifyNum indicates the service exposes an instance count flag
	// (`--<FlagPrefix>`), stored into proc.Config.Num.
	AllowModifyNum bool
	// MaxNum is a hard upper bound for proc.Config.Num at boot time.
	//
	// A value <= 0 means "no explicit limit". When > 0 and the requested Num
	// exceeds it, boot option validation fails.
	MaxNum int
	// DefaultNum decides the default instance count when the count flag isn't
	// explicitly set.
	DefaultNum func(ctx BootContext) int
	// AllowModifyHost indicates the service exposes a host override flag
	// (`--<FlagPrefix>.host`), stored into proc.Config.Host.
	AllowModifyHost bool
	// AllowModifyPort indicates the service exposes a port override flag
	// (`--<FlagPrefix>.port`), stored into proc.Config.Port.
	//
	// The meaning of "port override" is service-specific, but the general
	// convention is: 0 means "use the default port allocation logic".
	AllowModifyPort bool
	// DefaultPort is the default value used when registering the port flag.
	//
	// Most services leave this as 0 so "unset" remains distinguishable; a non-zero
	// value is useful when a service wants to surface a concrete default port in
	// help output.
	DefaultPort int
	// AllowModifyConfig indicates the service exposes a config file path flag
	// (`--<FlagPrefix>.config`), stored into proc.Config.ConfigPath.
	AllowModifyConfig bool
	// AllowModifyBinPath indicates the service exposes a binary path override flag
	// (`--<FlagPrefix>.binpath`), stored into proc.Config.BinPath.
	//
	// When a user provides a BinPath, playground will not resolve/download the
	// repository component for this service.
	AllowModifyBinPath bool
	// AllowModifyTimeout indicates the service exposes a ready-wait timeout flag
	// (`--<FlagPrefix>.timeout`), stored into proc.Config.UpTimeout (seconds).
	//
	// This timeout is used by instances that implement proc.ReadyWaiter.
	AllowModifyTimeout bool
	// DefaultTimeout is the default value used when registering the timeout flag.
	//
	// A value <= 0 means "no limit" (wait indefinitely), matching the semantics
	// of proc.Config.UpTimeout.
	DefaultTimeout int
	// AllowModifyVersion indicates the service exposes a per-service version constraint
	// override flag (`--<FlagPrefix>.version`), stored into proc.Config.Version.
	//
	// When false, proc.Config.Version is ignored and the global boot version is
	// used instead.
	AllowModifyVersion bool

	// AllowScaleOut indicates whether the service supports adding new instances in a
	// running playground (via the scale-out command path).
	AllowScaleOut bool

	// DefaultXXXFrom copies the value from another service when the destination
	// flag is not explicitly set.
	//
	// These are used to express "inherit from another service" behavior while
	// keeping per-service flags consistent. For example, a `*.system` service may
	// reuse the main service's binary path unless the user explicitly overrides
	// it.
	DefaultBinPathFrom    proc.ServiceID
	DefaultConfigPathFrom proc.ServiceID
	// DefaultHostFrom copies proc.Config.Host from another service.
	//
	// It is only applied when this service exposes the host flag and the host
	// flag is not explicitly set.
	DefaultHostFrom proc.ServiceID
	// DefaultPortFrom copies proc.Config.Port from another service.
	//
	// It is only applied when this service exposes the port flag and the port
	// flag is not explicitly set.
	DefaultPortFrom proc.ServiceID
	// DefaultTimeoutFrom copies proc.Config.UpTimeout from another service.
	//
	// This is typically used for services that do not expose their own timeout
	// flag but should follow another service's timeout value.
	DefaultTimeoutFrom proc.ServiceID

	// IsEnabled decides whether this service is enabled for the current boot
	// context.
	//
	// When nil or false, the service will not be included in the boot plan (so
	// no instances will be created at boot time and scale-out is disallowed).
	IsEnabled func(ctx BootContext) bool
	// PlanConfig returns the proc.Config snapshot used during planning when the
	// service is enabled.
	//
	// When nil, planning falls back to the config stored in BootOptions.Services.
	// This is useful for "internal" services that have no flags but should still
	// be started with a deterministic config.
	PlanConfig func(ctx BootContext) proc.Config

	// IsCritical marks a service as "critical" for the current boot context.
	//
	// When true and the planned instance count is > 0, the controller will treat
	// this service as required: if the number of running instances drops below
	// the required minimum, playground will trigger auto shutdown.
	IsCritical func(ctx BootContext) bool

	// VersionBind transforms the selected base version (boot version or per-service
	// override) before resolving repository components.
	//
	// It is typically used when a service is not available in some version
	// variants (e.g. NextGen suffix) and should fall back to the base TiDB
	// version.
	VersionBind VersionBindFunc
}

type Spec struct {
	ServiceID proc.ServiceID
	NewProc   func(rt ControllerRuntime, params NewProcParams) (proc.Process, error)

	Catalog Catalog

	// StartAfter declares boot-time ordering dependencies.
	//
	// Playground waits until at least one instance of each listed service becomes
	// ready (or is considered ready when it has no explicit ready check) before
	// starting this service.
	StartAfter []proc.ServiceID

	// ScaleInHook runs before the generic "expect-exit + SIGQUIT" path.
	//
	// If it returns async=true, the hook is responsible for arranging the actual
	// stop/removal (e.g. tombstone watchers), and the generic path is skipped.
	ScaleInHook ScaleInHookFunc

	// PostScaleOut is invoked after a scale-out instance is started successfully.
	PostScaleOut PostScaleOutFunc
}

// specs is intentionally treated as immutable after init() finishes.
// All registrations happen during package init, and runtime only performs reads.
var specs = make(map[proc.ServiceID]Spec)

func Register(spec Spec) error {
	if spec.ServiceID == "" {
		return fmt.Errorf("serviceID is empty")
	}
	if spec.NewProc == nil {
		return fmt.Errorf("service %s newProc is nil", spec.ServiceID)
	}
	if _, ok := specs[spec.ServiceID]; ok {
		return fmt.Errorf("duplicate service spec: %s", spec.ServiceID)
	}
	specs[spec.ServiceID] = spec
	return nil
}

func MustRegister(spec Spec) {
	if err := Register(spec); err != nil {
		panic(err.Error())
	}
}

func SpecFor(serviceID proc.ServiceID) (Spec, bool) {
	spec, ok := specs[serviceID]
	return spec, ok
}

func AllSpecs() []Spec {
	if len(specs) == 0 {
		return nil
	}
	serviceIDs := make([]proc.ServiceID, 0, len(specs))
	for id := range specs {
		serviceIDs = append(serviceIDs, id)
	}
	slices.SortStableFunc(serviceIDs, func(a, b proc.ServiceID) int {
		return strings.Compare(a.String(), b.String())
	})
	out := make([]Spec, 0, len(serviceIDs))
	for _, id := range serviceIDs {
		out = append(out, specs[id])
	}
	return out
}

func ProcsOf[T proc.Process](rt Runtime, serviceIDs ...proc.ServiceID) []T {
	if rt == nil || len(serviceIDs) == 0 {
		return nil
	}

	var out []T
	for _, serviceID := range serviceIDs {
		list := rt.Procs(serviceID)
		for _, inst := range list {
			v, ok := inst.(T)
			if !ok {
				panic(fmt.Sprintf("service %s has instance %T, want %T", serviceID, inst, *new(T)))
			}
			out = append(out, v)
		}
	}
	return out
}

type asyncScaleInStopEvent struct {
	serviceID   proc.ServiceID
	inst        proc.Process
	stopMessage string
}

func (e asyncScaleInStopEvent) Handle(rt ControllerRuntime) {
	if rt == nil || e.inst == nil {
		return
	}

	if !rt.RemoveProc(e.serviceID, e.inst) {
		return
	}

	if e.stopMessage != "" {
		fmt.Fprintln(rt.TermWriter(), e.stopMessage)
	}

	pid := 0
	if info := e.inst.Info(); info != nil && info.Proc != nil {
		pid = info.Proc.Pid()
	}
	rt.ExpectExitPID(pid)
	if pid > 0 {
		if err := syscall.Kill(pid, syscall.SIGQUIT); err != nil {
			fmt.Fprintln(rt.TermWriter(), err)
		}
	}

	rt.OnProcsChanged()
}

func watchAsyncScaleInStop(rt Runtime, interval time.Duration, probe func() (done bool, err error), evt asyncScaleInStopEvent) {
	if rt == nil || probe == nil {
		return
	}

	pollUntil(rt, interval, 0, probe, func() {
		rt.EmitEvent(evt)
	})
}

func pollUntil(rt Runtime, interval, timeout time.Duration, probe func() (done bool, err error), onDone func()) {
	if rt == nil || probe == nil || onDone == nil {
		return
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}

	start := time.Now()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if rt.Stopping() {
			return
		}

		done, err := probe()
		if err != nil {
			fmt.Fprintln(rt.TermWriter(), err)
		}
		if done {
			onDone()
			return
		}
		if timeout > 0 && time.Since(start) >= timeout {
			fmt.Fprintln(rt.TermWriter(), "timeout waiting for scale-in")
			return
		}
		<-ticker.C
	}
}

func pdClient(rt Runtime) (*api.PDClient, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	if len(pds) == 0 {
		return nil, fmt.Errorf("no pd instance available")
	}
	addrs := make([]string, 0, len(pds))
	for _, pd := range pds {
		addrs = append(addrs, pd.Addr())
	}
	ctx := context.WithValue(context.Background(), logprinter.ContextKeyLogger, logprinter.NewLogger(""))
	return api.NewPDClient(ctx, addrs, 10*time.Second, nil), nil
}

func binlogClient(rt Runtime) (*api.BinlogClient, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	if len(pds) == 0 {
		return nil, fmt.Errorf("no pd instance available")
	}
	addrs := make([]string, 0, len(pds))
	for _, pd := range pds {
		addrs = append(addrs, pd.Addr())
	}
	return api.NewBinlogClient(addrs, 5*time.Second, nil)
}

func dmMasterClient(rt Runtime) (*api.DMMasterClient, error) {
	masters := ProcsOf[*proc.DMMaster](rt, proc.ServiceDMMaster)
	if len(masters) == 0 {
		return nil, fmt.Errorf("no dm-master instance available")
	}
	addrs := make([]string, 0, len(masters))
	for _, master := range masters {
		addrs = append(addrs, master.Addr())
	}
	return api.NewDMMasterClient(addrs, 5*time.Second, nil), nil
}

func allocPort(host string, configured, defaultBase, portOffset int) int {
	base := configured
	if base <= 0 {
		base = defaultBase
	}
	return utils.MustGetFreePort(host, base, portOffset)
}
