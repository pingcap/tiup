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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/cheynewallace/tabby"
	"github.com/fatih/color"
	"github.com/pingcap/errors"
	"github.com/pingcap/failpoint"
	"github.com/pingcap/tiup/components/playground/instance"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
)

// The deadline a process should quit after receives kill signal
const killDeadline = time.Second * 16

// Playground represent the playground of a cluster.
type Playground struct {
	booted      bool
	curSig      int32
	bootOptions *bootOptions
	port        int

	pds      []*instance.PDInstance
	tikvs    []*instance.TiKVInstance
	tidbs    []*instance.TiDBInstance
	tiflashs []*instance.TiFlashInstance
	pumps    []*instance.Pump
	drainers []*instance.Drainer

	idAlloc        map[string]int
	instanceWaiter errgroup.Group

	monitor *monitor
}

// MonitorInfo represent the monitor
type MonitorInfo struct {
	IP         string `json:"ip"`
	Port       int    `json:"port"`
	BinaryPath string `json:"binary_path"`
}

// NewPlayground create a Playground instance.
func NewPlayground(port int) *Playground {
	return &Playground{
		port:    port,
		idAlloc: make(map[string]int),
	}
}

func (p *Playground) allocID(componentID string) int {
	id := p.idAlloc[componentID]
	p.idAlloc[componentID] = id + 1
	return id
}

func (p *Playground) handleDisplay(r io.Writer) (err error) {
	w := tabwriter.NewWriter(r, 0, 0, 2, ' ', 0)
	t := tabby.NewCustom(w)

	// TODO add more info.
	header := []interface{}{"Pid", "Role", "Uptime"}
	t.AddHeader(header...)

	err = p.WalkInstances(func(componentID string, ins instance.Instance) error {
		row := make([]interface{}, len(header))
		row[0] = strconv.Itoa(ins.Pid())
		row[1] = componentID
		row[2] = ins.Uptime()
		t.AddLine(row...)
		return nil
	})

	if err != nil {
		return errors.AddStack(err)
	}

	t.Print()
	return nil
}

var timeoutOpt = &clusterutil.RetryOption{
	Timeout: time.Second * 15,
	Delay:   time.Second * 5,
}

func (p *Playground) binlogClient() (*api.BinlogClient, error) {
	var addrs []string
	for _, inst := range p.pds {
		addrs = append(addrs, inst.Addr())
	}

	return api.NewBinlogClient(addrs, nil)
}

func (p *Playground) pdClient() *api.PDClient {
	var addrs []string
	for _, inst := range p.pds {
		addrs = append(addrs, inst.Addr())
	}

	return api.NewPDClient(addrs, 10*time.Second, nil)
}

func (p *Playground) killKVIfTombstone(inst *instance.TiKVInstance) {
	defer logIfErr(p.renderSDFile())

	for {
		tombstone, err := p.pdClient().IsTombStone(inst.Addr())
		if err != nil {
			fmt.Println(err)
		}

		if tombstone {
			for i, e := range p.tikvs {
				if e == inst {
					fmt.Printf("stop tombstone tikv %s\n", inst.Addr())
					err = syscall.Kill(inst.Pid(), syscall.SIGQUIT)
					if err != nil {
						fmt.Println(err)
					}
					p.tikvs = append(p.tikvs[:i], p.tikvs[i+1:]...)
					return
				}
			}
		}

		time.Sleep(time.Second * 5)
	}
}

func (p *Playground) removePumpWhenTombstone(c *api.BinlogClient, inst *instance.Pump) {
	defer logIfErr(p.renderSDFile())

	for {
		tombstone, err := c.IsPumpTombstone(inst.NodeID())
		if err != nil {
			fmt.Println(err)
		}

		if tombstone {
			for i, e := range p.pumps {
				if e == inst {
					fmt.Printf("pump already offline %s\n", inst.Addr())
					p.pumps = append(p.pumps[:i], p.pumps[i+1:]...)
					return
				}
			}
		}

		time.Sleep(time.Second * 5)
	}
}

func (p *Playground) removeDrainerWhenTombstone(c *api.BinlogClient, inst *instance.Drainer) {
	defer logIfErr(p.renderSDFile())

	for {
		tombstone, err := c.IsDrainerTombstone(inst.NodeID())
		if err != nil {
			fmt.Println(err)
		}

		if tombstone {
			for i, e := range p.drainers {
				if e == inst {
					fmt.Printf("drainer already offline %s\n", inst.Addr())
					p.drainers = append(p.drainers[:i], p.drainers[i+1:]...)
					return
				}
			}
		}

		time.Sleep(time.Second * 5)
	}
}

func (p *Playground) killFlashIfTombstone(inst *instance.TiFlashInstance) {
	defer logIfErr(p.renderSDFile())

	for {
		tombstone, err := p.pdClient().IsTombStone(inst.Addr())
		if err != nil {
			fmt.Println(err)
		}

		if tombstone {
			for i, e := range p.tiflashs {
				if e == inst {
					fmt.Printf("stop tombstone tiflash %s\n", inst.Addr())
					err = syscall.Kill(inst.Pid(), syscall.SIGQUIT)
					if err != nil {
						fmt.Println(err)
					}
					p.tiflashs = append(p.tiflashs[:i], p.tiflashs[i+1:]...)
					return
				}
			}
		}

		time.Sleep(time.Second * 5)
	}
}

func (p *Playground) handleScaleIn(w io.Writer, pid int) error {
	var cid string
	var inst instance.Instance
	err := p.WalkInstances(func(wcid string, winst instance.Instance) error {
		if winst.Pid() == pid {
			cid = wcid
			inst = winst
		}
		return nil
	})
	if err != nil {
		return errors.AddStack(err)
	}

	if inst == nil {
		fmt.Fprintf(w, "no instance with id: %d\n", pid)
		return nil
	}

	switch cid {
	case "pd":
		for i := 0; i < len(p.pds); i++ {
			if p.pds[i].Pid() == pid {
				inst := p.pds[i]
				err := p.pdClient().DelPD(inst.Name(), timeoutOpt)
				if err != nil {
					return errors.AddStack(err)
				}
				p.pds = append(p.pds[:i], p.pds[i+1:]...)
			}
		}
	case "tikv":
		for i := 0; i < len(p.tikvs); i++ {
			if p.tikvs[i].Pid() == pid {
				inst := p.tikvs[i]
				err := p.pdClient().DelStore(inst.Addr(), timeoutOpt)
				if err != nil {
					return errors.AddStack(err)
				}

				go p.killKVIfTombstone(inst)
				fmt.Fprintf(w, "tikv will be stop when tombstone\n")
				return nil
			}
		}
	case "tidb":
		for i := 0; i < len(p.tidbs); i++ {
			if p.tidbs[i].Pid() == pid {
				p.tidbs = append(p.tidbs[:i], p.tidbs[i+1:]...)
			}
		}
	case "tiflash":
		for i := 0; i < len(p.tiflashs); i++ {
			if p.tiflashs[i].Pid() == pid {
				inst := p.tiflashs[i]
				err := p.pdClient().DelStore(inst.Addr(), timeoutOpt)
				if err != nil {
					return errors.AddStack(err)
				}

				go p.killFlashIfTombstone(inst)
				fmt.Fprintf(w, "tiflash will be stop when tombstone\n")
				return nil
			}
		}
	case "pump":
		for i := 0; i < len(p.pumps); i++ {
			if p.pumps[i].Pid() == pid {
				inst := p.pumps[i]

				c, err := p.binlogClient()
				if err != nil {
					return errors.AddStack(err)
				}
				err = c.OfflinePump(inst.Addr(), inst.NodeID())
				if err != nil {
					return errors.AddStack(err)
				}

				go p.removePumpWhenTombstone(c, inst)
				fmt.Fprintf(w, "pump will be stop when offline\n")
				return nil
			}
		}
	case "drainer":
		for i := 0; i < len(p.drainers); i++ {
			if p.drainers[i].Pid() == pid {
				inst := p.drainers[i]

				c, err := p.binlogClient()
				if err != nil {
					return errors.AddStack(err)
				}
				err = c.OfflineDrainer(inst.Addr(), inst.NodeID())
				if err != nil {
					return errors.AddStack(err)
				}

				go p.removeDrainerWhenTombstone(c, inst)
				fmt.Fprintf(w, "drainer will be stop when offline\n")
				return nil
			}
		}
	default:
		fmt.Fprintf(w, "unknown component in scale in: %s", cid)
		return nil
	}

	err = syscall.Kill(pid, syscall.SIGQUIT)
	if err != nil {
		return errors.AddStack(err)
	}

	logIfErr(p.renderSDFile())

	fmt.Fprintf(w, "scale in %s success\n", cid)

	return nil
}

func (p *Playground) sanitizeConfig(boot instance.Config, cfg *instance.Config) error {
	if cfg.BinPath == "" {
		cfg.BinPath = boot.BinPath
	}
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = boot.ConfigPath
	}
	if cfg.Host == "" {
		cfg.Host = boot.ConfigPath
	}

	path, err := getAbsolutePath(cfg.ConfigPath)
	if err != nil {
		return err
	}
	cfg.ConfigPath = path
	return nil
}

func (p *Playground) sanitizeComponentConfig(cid string, cfg *instance.Config) error {
	switch cid {
	case "pd":
		return p.sanitizeConfig(p.bootOptions.pd, cfg)
	case "tikv":
		return p.sanitizeConfig(p.bootOptions.tikv, cfg)
	case "tidb":
		return p.sanitizeConfig(p.bootOptions.tidb, cfg)
	case "tiflash":
		return p.sanitizeConfig(p.bootOptions.tiflash, cfg)
	case "pump":
		return p.sanitizeConfig(p.bootOptions.pump, cfg)
	case "drainer":
		return p.sanitizeConfig(p.bootOptions.drainer, cfg)
	default:
		return fmt.Errorf("unknow %s in sanitizeConfig", cid)
	}
}

func (p *Playground) startInstance(ctx context.Context, inst instance.Instance) error {
	fmt.Printf("Start %s instance...\n", inst.Component())
	err := inst.Start(context.Background(), v0manifest.Version(p.bootOptions.version))
	p.addWaitInstance(inst)
	return errors.AddStack(err)
}

func (p *Playground) addWaitInstance(inst instance.Instance) {
	p.instanceWaiter.Go(func() error {
		err := inst.Wait(context.TODO())
		if err != nil && atomic.LoadInt32(&p.curSig) == 0 {
			fmt.Print(color.RedString("%s quit: %s\n", inst.Component(), err.Error()))
			if lines, _ := utils.TailN(inst.LogFile(), 10); len(lines) > 0 {
				for _, line := range lines {
					fmt.Println(line)
				}
				fmt.Print(color.YellowString("...\ncheck detail log from: %s\n", inst.LogFile()))
			}
		}
		return err
	})
}

func (p *Playground) handleScaleOut(w io.Writer, cmd *Command) error {
	// Ignore Config.Num, alway one command as scale out one instance.
	err := p.sanitizeComponentConfig(cmd.ComponentID, &cmd.Config)
	if err != nil {
		return err
	}
	inst, err := p.addInstance(cmd.ComponentID, cmd.Config)
	if err != nil {
		return errors.AddStack(err)
	}

	err = p.startInstance(context.Background(), inst)
	if err != nil {
		return errors.AddStack(err)
	}

	logIfErr(p.renderSDFile())

	return nil
}

func (p *Playground) handleCommand(cmd *Command, w io.Writer) error {
	fmt.Printf("receive command: %s\n", cmd.CommandType)
	switch cmd.CommandType {
	case DisplayCommandType:
		return p.handleDisplay(w)
	case ScaleInCommandType:
		return p.handleScaleIn(w, cmd.PID)
	case ScaleOutCommandType:
		return p.handleScaleOut(w, cmd)
	}

	return nil
}

func (p *Playground) listenAndServeHTTP() error {
	http.HandleFunc("/command", p.commandHandler)
	return http.ListenAndServe(":"+strconv.Itoa(p.port), nil)
}

func (p *Playground) commandHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(403)
		fmt.Fprintln(w, err)
		return
	}

	cmd := new(Command)
	err = json.Unmarshal(data, cmd)
	if err != nil {
		w.WriteHeader(403)
		fmt.Fprintln(w, err)
		return
	}

	err = p.handleCommand(cmd, w)
	if err != nil {
		w.WriteHeader(403)
		fmt.Fprintln(w, err)
	}
}

// RWalkInstances work like WalkInstances, but in the reverse order.
func (p *Playground) RWalkInstances(fn func(componentID string, ins instance.Instance) error) error {
	var ids []string
	var instances []instance.Instance

	_ = p.WalkInstances(func(id string, ins instance.Instance) error {
		ids = append(ids, id)
		instances = append(instances, ins)
		return nil
	})

	for i := len(ids); i > 0; i-- {
		err := fn(ids[i-1], instances[i-1])
		if err != nil {
			return errors.AddStack(err)
		}
	}
	return nil
}

// WalkInstances call fn for every intance and stop if return not nil.
func (p *Playground) WalkInstances(fn func(componentID string, ins instance.Instance) error) error {
	for _, ins := range p.pds {
		err := fn("pd", ins)
		if err != nil {
			return errors.AddStack(err)
		}
	}
	for _, ins := range p.tikvs {
		err := fn("tikv", ins)
		if err != nil {
			return errors.AddStack(err)
		}
	}

	for _, ins := range p.pumps {
		err := fn("pump", ins)
		if err != nil {
			return errors.AddStack(err)
		}
	}

	for _, ins := range p.tidbs {
		err := fn("tidb", ins)
		if err != nil {
			return errors.AddStack(err)
		}
	}

	for _, ins := range p.drainers {
		err := fn("drainer", ins)
		if err != nil {
			return errors.AddStack(err)
		}
	}

	for _, ins := range p.tiflashs {
		err := fn("tiflash", ins)
		if err != nil {
			return errors.AddStack(err)
		}
	}
	return nil
}

func (p *Playground) enableBinlog() bool {
	return p.bootOptions.pump.Num > 0
}

func (p *Playground) addInstance(componentID string, cfg instance.Config) (ins instance.Instance, err error) {
	if cfg.BinPath != "" {
		cfg.BinPath, err = getAbsolutePath(cfg.BinPath)
		if err != nil {
			return nil, err
		}
	}

	if cfg.ConfigPath != "" {
		cfg.ConfigPath, err = getAbsolutePath(cfg.ConfigPath)
		if err != nil {
			return nil, err
		}
	}

	dataDir := os.Getenv(localdata.EnvNameInstanceDataDir)
	if dataDir == "" {
		return nil, fmt.Errorf("cannot read environment variable %s", localdata.EnvNameInstanceDataDir)
	}

	id := p.allocID(componentID)
	dir := filepath.Join(dataDir, fmt.Sprintf("%s-%d", componentID, id))
	// look more like listen ip?
	host := p.bootOptions.host
	if cfg.Host != "" {
		host = cfg.Host
	}

	switch componentID {
	case "pd":
		inst := instance.NewPDInstance(cfg.BinPath, dir, host, cfg.ConfigPath, id)
		ins = inst
		if p.booted {
			inst.Join(p.pds)
			p.pds = append(p.pds, inst)
		} else {
			p.pds = append(p.pds, inst)
			for _, pd := range p.pds {
				pd.InitCluster(p.pds)
			}
		}
	case "tidb":
		inst := instance.NewTiDBInstance(cfg.BinPath, dir, host, cfg.ConfigPath, id, p.pds, p.enableBinlog())
		ins = inst
		p.tidbs = append(p.tidbs, inst)
	case "tikv":
		inst := instance.NewTiKVInstance(cfg.BinPath, dir, host, cfg.ConfigPath, id, p.pds)
		ins = inst
		p.tikvs = append(p.tikvs, inst)
	case "tiflash":
		inst := instance.NewTiFlashInstance(cfg.BinPath, dir, host, cfg.ConfigPath, id, p.pds, p.tidbs)
		ins = inst
		p.tiflashs = append(p.tiflashs, inst)
	case "pump":
		inst := instance.NewPump(cfg.BinPath, dir, host, cfg.ConfigPath, id, p.pds)
		ins = inst
		p.pumps = append(p.pumps, inst)
	case "drainer":
		inst := instance.NewDrainer(cfg.BinPath, dir, host, cfg.ConfigPath, id, p.pds)
		ins = inst
		p.drainers = append(p.drainers, inst)
	default:
		return nil, errors.Errorf("unknow component: %s", componentID)
	}

	return
}

func (p *Playground) bootCluster(env *environment.Environment, options *bootOptions) error {
	for _, cfg := range []*instance.Config{&options.pd, &options.tidb, &options.tikv, &options.tiflash, &options.pump, &options.drainer} {
		path, err := getAbsolutePath(cfg.ConfigPath)
		if err != nil {
			return errors.Annotatef(err, "cannot eval absolute directory: %s", cfg.ConfigPath)
		}
		cfg.ConfigPath = path
	}

	p.bootOptions = options

	if options.pd.Num < 1 || options.tidb.Num < 1 || options.tikv.Num < 1 {
		return fmt.Errorf("all components count must be great than 0 (tidb=%v, tikv=%v, pd=%v)",
			options.pd.Num, options.tikv.Num, options.pd.Num)
	}

	if options.version == "" {
		version, _, err := env.V1Repository().LatestStableVersion("tidb", false)
		if err != nil {
			return err
		}
		options.version = version.String()

		fmt.Println(color.YellowString(`Use the latest stable version: %s

    Specify version manually:   tiup playground <version>
    The stable version:         tiup playground v4.0.0
    The nightly version:        tiup playground nightly
`, options.version))
	}

	if options.version != "nightly" {
		if semver.Compare("v3.1.0", options.version) > 0 && options.tiflash.Num != 0 {
			fmt.Println(color.YellowString("Warning: current version %s doesn't support TiFlash", options.version))
			options.tiflash.Num = 0
		} else if runtime.GOOS == "darwin" && semver.Compare("v4.0.0", options.version) > 0 {
			// only runs tiflash on version later than v4.0.0 when executing on darwin
			fmt.Println(color.YellowString("Warning: current version %s doesn't support TiFlash on darwin", options.version))
			options.tiflash.Num = 0
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < options.pd.Num; i++ {
		_, err := p.addInstance("pd", options.pd)
		if err != nil {
			return errors.AddStack(err)
		}
	}
	for i := 0; i < options.tikv.Num; i++ {
		_, err := p.addInstance("tikv", options.tikv)
		if err != nil {
			return errors.AddStack(err)
		}
	}
	for i := 0; i < options.pump.Num; i++ {
		_, err := p.addInstance("pump", options.pump)
		if err != nil {
			return errors.AddStack(err)
		}
	}
	for i := 0; i < options.tidb.Num; i++ {
		_, err := p.addInstance("tidb", options.tidb)
		if err != nil {
			return errors.AddStack(err)
		}
	}
	for i := 0; i < options.drainer.Num; i++ {
		_, err := p.addInstance("drainer", options.drainer)
		if err != nil {
			return errors.AddStack(err)
		}
	}
	for i := 0; i < options.tiflash.Num; i++ {
		_, err := p.addInstance("tiflash", options.tiflash)
		if err != nil {
			return errors.AddStack(err)
		}
	}

	fmt.Println("Playground Bootstrapping...")

	var monitorCmd *exec.Cmd
	var grafana *grafana
	var monitorInfo *MonitorInfo
	if options.monitor {
		var err error
		if monitorCmd, monitorInfo, grafana, err = p.bootMonitor(ctx, env); err != nil {
			return errors.AddStack(err)
		}
	}

	anyPumpReady := false
	// Start all instance except tiflash.
	err := p.WalkInstances(func(cid string, ins instance.Instance) error {
		if cid == "tiflash" {
			return nil
		}

		err := p.startInstance(ctx, ins)
		if err != nil {
			return err
		}

		// if no any pump, tidb will quit right away.
		if cid == "pump" && !anyPumpReady {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
			err = ins.(*instance.Pump).Ready(ctx)
			cancel()
			if err != nil {
				return err
			}
			anyPumpReady = true
		}

		return nil
	})
	if err != nil {
		return errors.AddStack(err)
	}

	p.booted = true

	var succ []string
	for _, db := range p.tidbs {
		if s := checkDB(db.Addr()); s {
			succ = append(succ, db.Addr())
		}
	}

	if len(succ) > 0 {
		// start TiFlash after at least one TiDB is up.
		var lastErr error

		var endpoints []string
		for _, pd := range p.pds {
			endpoints = append(endpoints, pd.Addr())
		}
		pdClient := api.NewPDClient(endpoints, 10*time.Second, nil)

		// make sure TiKV are all up
		for _, kv := range p.tikvs {
			if err := checkStoreStatus(pdClient, "tikv", kv.StoreAddr()); err != nil {
				lastErr = err
				break
			}
		}

		if lastErr == nil {
			for _, flash := range p.tiflashs {
				if err := p.startInstance(ctx, flash); err != nil {
					lastErr = err
					break
				}
			}
		}

		if lastErr == nil {
			// check if all TiFlash is up
			for _, flash := range p.tiflashs {
				cmd := flash.Cmd()
				if cmd == nil {
					lastErr = errors.Errorf("tiflash %s initialize command failed", flash.StoreAddr())
					break
				}
				if state := cmd.ProcessState; state != nil && state.Exited() {
					lastErr = errors.Errorf("tiflash process exited with code: %d", state.ExitCode())
					break
				}
				if err := checkStoreStatus(pdClient, "tiflash", flash.StoreAddr()); err != nil {
					lastErr = err
					break
				}
			}
		}

		if lastErr != nil {
			fmt.Println(color.RedString("TiFlash failed to start. %s", lastErr))
		} else {
			fmt.Println(color.GreenString("CLUSTER START SUCCESSFULLY, Enjoy it ^-^"))
			for _, dbAddr := range succ {
				ss := strings.Split(dbAddr, ":")
				fmt.Println(color.GreenString("To connect TiDB: mysql --host %s --port %s -u root", ss[0], ss[1]))
			}
		}
	}

	if pdAddr := p.pds[0].Addr(); hasDashboard(pdAddr) {
		fmt.Println(color.GreenString("To view the dashboard: http://%s/dashboard", pdAddr))
	}

	if monitorInfo != nil && len(p.pds) != 0 {
		client, err := newEtcdClient(p.pds[0].Addr())
		if err == nil && client != nil {
			promBinary, err := json.Marshal(monitorInfo)
			if err == nil {
				_, err = client.Put(context.TODO(), "/topology/prometheus", string(promBinary))
				if err != nil {
					fmt.Println("Set the PD metrics storage failed")
				}
				fmt.Print(color.GreenString("To view the Prometheus: http://%s:%d\n", monitorInfo.IP, monitorInfo.Port))
			}
		}
	}

	dumpDSN(p.tidbs)

	failpoint.Inject("terminateEarly", func() error {
		time.Sleep(20 * time.Second)

		fmt.Println("Early terminated via failpoint")

		extraCmds := []*exec.Cmd{}
		if grafana != nil {
			extraCmds = append(extraCmds, grafana.cmd)
		}
		if monitorCmd != nil {
			extraCmds = append(extraCmds, monitorCmd)
		}
		p.terminate(syscall.SIGKILL, extraCmds...)

		return nil
	})

	go func() {
		sc := make(chan os.Signal, 1)
		signal.Notify(sc,
			syscall.SIGHUP,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGQUIT)
		sig := (<-sc).(syscall.Signal)
		atomic.StoreInt32(&p.curSig, int32(sig))
		extraCmds := []*exec.Cmd{}
		if grafana != nil {
			extraCmds = append(extraCmds, grafana.cmd)
		}
		if monitorCmd != nil {
			extraCmds = append(extraCmds, monitorCmd)
		}
		go p.terminate(sig, extraCmds...)

		// If user try double ctrl+c, force quit
		sig = (<-sc).(syscall.Signal)
		atomic.StoreInt32(&p.curSig, int32(syscall.SIGKILL))
		if sig == syscall.SIGINT {
			p.terminate(syscall.SIGKILL, extraCmds...)
		}
	}()

	go func() {
		// fmt.Printf("serve at :%d\n", p.port)
		err := p.listenAndServeHTTP()
		if err != nil {
			fmt.Printf("listenAndServeHTTP quit: %s\n", err)
		}
	}()

	logIfErr(p.renderSDFile())

	if grafana != nil {
		p.instanceWaiter.Go(func() error {
			err := grafana.cmd.Wait()
			if err != nil && atomic.LoadInt32(&p.curSig) == 0 {
				fmt.Printf("Grafana quit: %v\n", err)
			}
			return err
		})
		fmt.Print(color.GreenString("To view the Grafana: http://%s:%d\n", grafana.host, grafana.port))
	}

	// Wait all instance quit and return the first non-nil err.
	err = p.instanceWaiter.Wait()
	if err != nil && atomic.LoadInt32(&p.curSig) == 0 {
		return err
	}

	if monitorCmd != nil {
		if err := monitorCmd.Wait(); err != nil && atomic.LoadInt32(&p.curSig) == 0 {
			fmt.Println("Monitor system wait failed", err)
		}
	}

	return nil
}

func (p *Playground) terminate(sig syscall.Signal, extraCmds ...*exec.Cmd) {
	_ = p.RWalkInstances(func(_ string, inst instance.Instance) error {
		if sig != syscall.SIGINT {
			_ = syscall.Kill(inst.Pid(), sig)
		}
		if sig == syscall.SIGKILL {
			fmt.Printf("Force %s(%d) to quit...\n", inst.Component(), inst.Pid())
		} else if atomic.LoadInt32(&p.curSig) == int32(sig) { // In case of double ctr+c
			fmt.Printf("Wait %s(%d) to quit...\n", inst.Component(), inst.Pid())
		}
		ctx, cancel := context.WithTimeout(context.Background(), killDeadline)
		defer cancel()
		if err := inst.Wait(ctx); err == utils.ErrorWaitTimeout {
			_ = syscall.Kill(inst.Pid(), syscall.SIGKILL)
		}
		return nil
	})
	for _, cmd := range extraCmds {
		if sig != syscall.SIGINT {
			_ = syscall.Kill(cmd.Process.Pid, sig)
		}
		ctx, cancel := context.WithTimeout(context.Background(), killDeadline)
		defer cancel()
		if err := utils.WaitContext(ctx, cmd); err == utils.ErrorWaitTimeout {
			_ = syscall.Kill(cmd.Process.Pid, syscall.SIGKILL)
		}
	}
}

func (p *Playground) renderSDFile() error {
	// we not start monitor at all.
	if p.monitor == nil {
		return nil
	}

	cid2targets := make(map[string][]string)

	_ = p.WalkInstances(func(cid string, inst instance.Instance) error {
		targets := cid2targets[cid]
		targets = append(targets, inst.StatusAddrs()...)
		cid2targets[cid] = targets
		return nil
	})

	err := p.monitor.renderSDFile(cid2targets)
	if err != nil {
		return errors.AddStack(err)
	}

	return nil
}

func (p *Playground) bootMonitor(ctx context.Context, env *environment.Environment) (*exec.Cmd, *MonitorInfo, *grafana, error) {
	options := p.bootOptions
	monitorInfo := &MonitorInfo{}

	dataDir := os.Getenv(localdata.EnvNameInstanceDataDir)
	promDir := filepath.Join(dataDir, "prometheus")

	monitor := newMonitor()
	port, cmd, err := monitor.startMonitor(ctx, options.version, options.host, promDir)
	if err != nil {
		return nil, nil, nil, err
	}
	p.monitor = monitor

	monitorInfo.IP = options.host
	monitorInfo.BinaryPath = promDir
	monitorInfo.Port = port

	monitorCmd := cmd
	go func() {
		log, err := os.OpenFile(filepath.Join(promDir, "prom.log"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, os.ModePerm)
		if err != nil {
			fmt.Println("Monitor system start failed", err)
			return
		}
		defer log.Close()

		cmd.Stderr = log
		cmd.Stdout = os.Stdout

		// fmt.Println("Start Prometheus instance...")
		if err := cmd.Start(); err != nil {
			fmt.Println("Monitor system start failed", err)
			return
		}
	}()

	// set up grafana
	if err := installIfMissing(env.Profile(), "grafana", options.version); err != nil {
		return nil, nil, nil, errors.AddStack(err)
	}
	installPath, err := env.Profile().ComponentInstalledPath("grafana", v0manifest.Version(options.version))
	if err != nil {
		return nil, nil, nil, errors.AddStack(err)
	}

	dataDir = os.Getenv(localdata.EnvNameInstanceDataDir)
	grafanaDir := filepath.Join(dataDir, "grafana")

	cmd = exec.Command("cp", "-r", installPath, grafanaDir)
	err = cmd.Run()
	if err != nil {
		return nil, nil, nil, errors.AddStack(err)
	}

	dashboardDir := filepath.Join(grafanaDir, "dashboards")
	err = os.MkdirAll(dashboardDir, 0755)
	if err != nil {
		return nil, nil, nil, errors.AddStack(err)
	}

	// mv {grafanaDir}/*.json {grafanaDir}/dashboards/
	err = filepath.Walk(grafanaDir, func(path string, info os.FileInfo, err error) error {
		// skip scan sub directory
		if info.IsDir() && path != grafanaDir {
			return filepath.SkipDir
		}

		if strings.HasSuffix(info.Name(), ".json") {
			return os.Rename(path, filepath.Join(grafanaDir, "dashboards", info.Name()))
		}

		return nil
	})
	if err != nil {
		return nil, nil, nil, errors.AddStack(err)
	}

	err = replaceDatasource(dashboardDir, clusterName)
	if err != nil {
		return nil, nil, nil, errors.AddStack(err)
	}

	grafana := newGrafana(options.version, options.host)
	// fmt.Println("Start Grafana instance...")
	err = grafana.start(ctx, grafanaDir, fmt.Sprintf("http://%s:%d", monitorInfo.IP, monitorInfo.Port))
	if err != nil {
		return nil, nil, nil, errors.AddStack(err)
	}

	return monitorCmd, monitorInfo, grafana, nil
}

func logIfErr(err error) {
	if err != nil {
		fmt.Println(err)
	}
}
