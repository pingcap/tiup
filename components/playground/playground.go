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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/AstroProfundis/tabby"
	"github.com/fatih/color"
	"github.com/juju/ansiterm"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/instance"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/environment"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/tui/progress"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// The duration process need to quit gracefully, or we kill the process.
const forceKillAfterDuration = time.Second * 10

// Playground represent the playground of a cluster.
type Playground struct {
	dataDir string
	booted  bool
	// the latest receive signal
	curSig      int32
	bootOptions *BootOptions
	port        int

	pds              []*instance.PDInstance
	tikvs            []*instance.TiKVInstance
	tidbs            []*instance.TiDBInstance
	tiflashs         []*instance.TiFlashInstance
	ticdcs           []*instance.TiCDC
	pumps            []*instance.Pump
	drainers         []*instance.Drainer
	startedInstances []instance.Instance

	idAlloc        map[string]int
	instanceWaiter errgroup.Group

	// not nil iff we start the exec.Cmd successfully.
	// we should and can safely call wait() to make sure the process quit
	// before playground quit.
	monitor      *monitor
	ngmonitoring *ngMonitoring
	grafana      *grafana
}

// MonitorInfo represent the monitor
type MonitorInfo struct {
	IP         string `json:"ip"`
	Port       int    `json:"port"`
	BinaryPath string `json:"binary_path"`
}

// NewPlayground create a Playground instance.
func NewPlayground(dataDir string, port int) *Playground {
	return &Playground{
		dataDir: dataDir,
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
	w := ansiterm.NewTabWriter(r, 0, 0, 2, ' ', 0)
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
		return err
	}

	t.Print()
	return nil
}

var timeoutOpt = &utils.RetryOption{
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

	return api.NewPDClient(
		context.WithValue(context.TODO(), logprinter.ContextKeyLogger, log),
		addrs, 10*time.Second, nil,
	)
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
		tombstone, err := c.IsPumpTombstone(context.TODO(), inst.Addr())
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
		tombstone, err := c.IsDrainerTombstone(context.TODO(), inst.Addr())
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

func (p *Playground) killTiFlashIfTombstone(inst *instance.TiFlashInstance) {
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
		return err
	}

	if inst == nil {
		fmt.Fprintf(w, "no instance with id: %d\n", pid)
		return nil
	}

	switch cid {
	case spec.ComponentPD:
		for i := 0; i < len(p.pds); i++ {
			if p.pds[i].Pid() == pid {
				inst := p.pds[i]
				err := p.pdClient().DelPD(inst.Name(), timeoutOpt)
				if err != nil {
					return err
				}
				p.pds = append(p.pds[:i], p.pds[i+1:]...)
			}
		}
	case spec.ComponentTiKV:
		for i := 0; i < len(p.tikvs); i++ {
			if p.tikvs[i].Pid() == pid {
				inst := p.tikvs[i]
				err := p.pdClient().DelStore(inst.Addr(), timeoutOpt)
				if err != nil {
					return err
				}

				go p.killKVIfTombstone(inst)
				fmt.Fprintf(w, "tikv will be stop when tombstone\n")
				return nil
			}
		}
	case spec.ComponentTiDB:
		for i := 0; i < len(p.tidbs); i++ {
			if p.tidbs[i].Pid() == pid {
				p.tidbs = append(p.tidbs[:i], p.tidbs[i+1:]...)
			}
		}
	case spec.ComponentCDC:
		for i := 0; i < len(p.ticdcs); i++ {
			if p.ticdcs[i].Pid() == pid {
				p.ticdcs = append(p.ticdcs[:i], p.ticdcs[i+1:]...)
			}
		}
	case spec.ComponentTiFlash:
		for i := 0; i < len(p.tiflashs); i++ {
			if p.tiflashs[i].Pid() == pid {
				inst := p.tiflashs[i]
				err := p.pdClient().DelStore(inst.Addr(), timeoutOpt)
				if err != nil {
					return err
				}

				go p.killTiFlashIfTombstone(inst)
				fmt.Fprintf(w, "tiflash will be stop when tombstone\n")
				return nil
			}
		}
	case spec.ComponentPump:
		for i := 0; i < len(p.pumps); i++ {
			if p.pumps[i].Pid() == pid {
				inst := p.pumps[i]

				c, err := p.binlogClient()
				if err != nil {
					return err
				}
				err = c.OfflinePump(context.TODO(), inst.Addr())
				if err != nil {
					return err
				}

				go p.removePumpWhenTombstone(c, inst)
				fmt.Fprintf(w, "pump will be stop when offline\n")
				return nil
			}
		}
	case spec.ComponentDrainer:
		for i := 0; i < len(p.drainers); i++ {
			if p.drainers[i].Pid() == pid {
				inst := p.drainers[i]

				c, err := p.binlogClient()
				if err != nil {
					return err
				}
				err = c.OfflineDrainer(context.TODO(), inst.Addr())
				if err != nil {
					return err
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
		cfg.Host = boot.Host
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
	case spec.ComponentPD:
		return p.sanitizeConfig(p.bootOptions.PD, cfg)
	case spec.ComponentTiKV:
		return p.sanitizeConfig(p.bootOptions.TiKV, cfg)
	case spec.ComponentTiDB:
		return p.sanitizeConfig(p.bootOptions.TiDB, cfg)
	case spec.ComponentTiFlash:
		return p.sanitizeConfig(p.bootOptions.TiFlash, cfg)
	case spec.ComponentCDC:
		return p.sanitizeConfig(p.bootOptions.TiCDC, cfg)
	case spec.ComponentPump:
		return p.sanitizeConfig(p.bootOptions.Pump, cfg)
	case spec.ComponentDrainer:
		return p.sanitizeConfig(p.bootOptions.Drainer, cfg)
	default:
		return fmt.Errorf("unknown %s in sanitizeConfig", cid)
	}
}

func (p *Playground) startInstance(ctx context.Context, inst instance.Instance) error {
	version, err := environment.GlobalEnv().V1Repository().ResolveComponentVersion(inst.Component(), p.bootOptions.Version)
	if err != nil {
		return err
	}
	fmt.Printf("Start %s instance:%s\n", inst.Component(), version)
	err = inst.Start(ctx, version)
	if err != nil {
		return err
	}
	p.addWaitInstance(inst)
	return nil
}

func (p *Playground) addWaitInstance(inst instance.Instance) {
	p.startedInstances = append(p.startedInstances, inst)
	p.instanceWaiter.Go(func() error {
		err := inst.Wait()
		if err != nil && atomic.LoadInt32(&p.curSig) == 0 {
			fmt.Print(color.RedString("%s quit: %s\n", inst.Component(), err.Error()))
			if lines, _ := utils.TailN(inst.LogFile(), 10); len(lines) > 0 {
				for _, line := range lines {
					fmt.Println(line)
				}
				fmt.Print(color.YellowString("...\ncheck detail log from: %s\n", inst.LogFile()))
			}
		} else {
			fmt.Printf("%s quit\n", inst.Component())
		}
		return err
	})
}

func (p *Playground) handleScaleOut(w io.Writer, cmd *Command) error {
	// Ignore Config.Num, always one command as scale out one instance.
	err := p.sanitizeComponentConfig(cmd.ComponentID, &cmd.Config)
	if err != nil {
		return err
	}
	inst, err := p.addInstance(cmd.ComponentID, cmd.Config)
	if err != nil {
		return err
	}

	err = p.startInstance(
		context.WithValue(context.TODO(), logprinter.ContextKeyLogger, log),
		inst,
	)
	if err != nil {
		return err
	}

	if cmd.ComponentID == "tidb" {
		addr := p.tidbs[len(p.tidbs)-1].Addr()
		if checkDB(addr, cmd.UpTimeout) {
			ss := strings.Split(addr, ":")
			connectMsg := "To connect new added TiDB: mysql --comments --host %s --port %s -u root -p (no password)"
			fmt.Println(color.GreenString(connectMsg, ss[0], ss[1]))
			fmt.Fprintln(w, color.GreenString(connectMsg, ss[0], ss[1]))
		}
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

	data, err := io.ReadAll(r.Body)
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
			return err
		}
	}
	return nil
}

// WalkInstances call fn for every instance and stop if return not nil.
func (p *Playground) WalkInstances(fn func(componentID string, ins instance.Instance) error) error {
	for _, ins := range p.pds {
		err := fn(spec.ComponentPD, ins)
		if err != nil {
			return err
		}
	}
	for _, ins := range p.tikvs {
		err := fn(spec.ComponentTiKV, ins)
		if err != nil {
			return err
		}
	}

	for _, ins := range p.pumps {
		err := fn(spec.ComponentPump, ins)
		if err != nil {
			return err
		}
	}

	for _, ins := range p.tidbs {
		err := fn(spec.ComponentTiDB, ins)
		if err != nil {
			return err
		}
	}

	for _, ins := range p.ticdcs {
		err := fn(spec.ComponentCDC, ins)
		if err != nil {
			return err
		}
	}

	for _, ins := range p.drainers {
		err := fn(spec.ComponentDrainer, ins)
		if err != nil {
			return err
		}
	}

	for _, ins := range p.tiflashs {
		err := fn(spec.ComponentTiFlash, ins)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Playground) enableBinlog() bool {
	return p.bootOptions.Pump.Num > 0
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

	dataDir := p.dataDir

	id := p.allocID(componentID)
	dir := filepath.Join(dataDir, fmt.Sprintf("%s-%d", componentID, id))
	if err = os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	// look more like listen ip?
	host := p.bootOptions.Host
	if cfg.Host != "" {
		host = cfg.Host
	}

	switch componentID {
	case spec.ComponentPD:
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
	case spec.ComponentTiDB:
		inst := instance.NewTiDBInstance(cfg.BinPath, dir, host, cfg.ConfigPath, id, cfg.Port, p.pds, p.enableBinlog())
		ins = inst
		p.tidbs = append(p.tidbs, inst)
	case spec.ComponentTiKV:
		inst := instance.NewTiKVInstance(cfg.BinPath, dir, host, cfg.ConfigPath, id, p.pds)
		ins = inst
		p.tikvs = append(p.tikvs, inst)
	case spec.ComponentTiFlash:
		inst := instance.NewTiFlashInstance(cfg.BinPath, dir, host, cfg.ConfigPath, id, p.pds, p.tidbs)
		ins = inst
		p.tiflashs = append(p.tiflashs, inst)
	case spec.ComponentCDC:
		inst := instance.NewTiCDC(cfg.BinPath, dir, host, cfg.ConfigPath, id, p.pds)
		ins = inst
		p.ticdcs = append(p.ticdcs, inst)
	case spec.ComponentPump:
		inst := instance.NewPump(cfg.BinPath, dir, host, cfg.ConfigPath, id, p.pds)
		ins = inst
		p.pumps = append(p.pumps, inst)
	case spec.ComponentDrainer:
		inst := instance.NewDrainer(cfg.BinPath, dir, host, cfg.ConfigPath, id, p.pds)
		ins = inst
		p.drainers = append(p.drainers, inst)
	default:
		return nil, errors.Errorf("unknown component: %s", componentID)
	}

	return
}

func (p *Playground) waitAllTidbUp() []string {
	var succ []string
	if len(p.tidbs) > 0 {
		var wg sync.WaitGroup
		var appendMutex sync.Mutex
		bars := progress.NewMultiBar(color.YellowString("Waiting for tidb instances ready\n"))
		for _, db := range p.tidbs {
			wg.Add(1)
			prefix := color.YellowString(db.Addr())
			bar := bars.AddBar(prefix)
			go func(dbInst *instance.TiDBInstance) {
				defer wg.Done()
				if s := checkDB(dbInst.Addr(), options.TiDB.UpTimeout); s {
					{
						appendMutex.Lock()
						succ = append(succ, dbInst.Addr())
						appendMutex.Unlock()
					}
					bar.UpdateDisplay(&progress.DisplayProps{
						Prefix: prefix,
						Mode:   progress.ModeDone,
					})
				} else {
					bar.UpdateDisplay(&progress.DisplayProps{
						Prefix: prefix,
						Mode:   progress.ModeError,
					})
				}
			}(db)
		}
		bars.StartRenderLoop()
		wg.Wait()
		bars.StopRenderLoop()
	}
	return succ
}

func (p *Playground) waitAllTiFlashUp() {
	if len(p.tiflashs) > 0 {
		var endpoints []string
		for _, pd := range p.pds {
			endpoints = append(endpoints, pd.Addr())
		}
		pdClient := api.NewPDClient(
			context.WithValue(context.TODO(), logprinter.ContextKeyLogger, log),
			endpoints, 10*time.Second, nil,
		)

		var wg sync.WaitGroup
		bars := progress.NewMultiBar(color.YellowString("Waiting for tiflash instances ready\n"))
		for _, flash := range p.tiflashs {
			wg.Add(1)
			prefix := color.YellowString(flash.Addr())
			bar := bars.AddBar(prefix)
			go func(flashInst *instance.TiFlashInstance) {
				defer wg.Done()
				displayResult := &progress.DisplayProps{
					Prefix: prefix,
				}
				if cmd := flashInst.Cmd(); cmd == nil {
					displayResult.Mode = progress.ModeError
					displayResult.Suffix = "initialize command failed"
				} else if state := cmd.ProcessState; state != nil && state.Exited() {
					displayResult.Mode = progress.ModeError
					displayResult.Suffix = fmt.Sprintf("process exited with code: %d", state.ExitCode())
				} else if s := checkStoreStatus(pdClient, flashInst.Addr(), options.TiFlash.UpTimeout); !s {
					displayResult.Mode = progress.ModeError
					displayResult.Suffix = "failed to up after timeout"
				} else {
					displayResult.Mode = progress.ModeDone
				}
				bar.UpdateDisplay(displayResult)
			}(flash)
		}
		bars.StartRenderLoop()
		wg.Wait()
		bars.StopRenderLoop()
	}
}

func (p *Playground) bootCluster(ctx context.Context, env *environment.Environment, options *BootOptions) error {
	for _, cfg := range []*instance.Config{
		&options.PD,
		&options.TiDB,
		&options.TiKV,
		&options.TiFlash,
		&options.Pump,
		&options.Drainer,
	} {
		path, err := getAbsolutePath(cfg.ConfigPath)
		if err != nil {
			return errors.Annotatef(err, "cannot eval absolute directory: %s", cfg.ConfigPath)
		}
		cfg.ConfigPath = path
	}

	p.bootOptions = options

	if options.PD.Num < 1 || options.TiKV.Num < 1 {
		return fmt.Errorf("all components count must be great than 0 (tikv=%v, pd=%v)", options.TiKV.Num, options.PD.Num)
	}

	if !utils.Version(options.Version).IsNightly() {
		if semver.Compare(options.Version, "v3.1.0") < 0 && options.TiFlash.Num != 0 {
			fmt.Println(color.YellowString("Warning: current version %s doesn't support TiFlash", options.Version))
			options.TiFlash.Num = 0
		} else if runtime.GOOS == "darwin" && semver.Compare(options.Version, "v4.0.0") < 0 {
			// only runs tiflash on version later than v4.0.0 when executing on darwin
			fmt.Println(color.YellowString("Warning: current version %s doesn't support TiFlash on darwin", options.Version))
			options.TiFlash.Num = 0
		}
	}

	instances := []struct {
		comp string
		instance.Config
	}{
		{spec.ComponentPD, options.PD},
		{spec.ComponentTiKV, options.TiKV},
		{spec.ComponentPump, options.Pump},
		{spec.ComponentTiDB, options.TiDB},
		{spec.ComponentCDC, options.TiCDC},
		{spec.ComponentDrainer, options.Drainer},
		{spec.ComponentTiFlash, options.TiFlash},
	}

	for _, inst := range instances {
		for i := 0; i < inst.Num; i++ {
			_, err := p.addInstance(inst.comp, inst.Config)
			if err != nil {
				return err
			}
		}
	}

	fmt.Println("Playground Bootstrapping...")

	anyPumpReady := false
	// Start all instance except tiflash.
	err := p.WalkInstances(func(cid string, ins instance.Instance) error {
		if cid == spec.ComponentTiFlash {
			return nil
		}

		err := p.startInstance(ctx, ins)
		if err != nil {
			return err
		}

		// if no any pump, tidb will quit right away.
		if cid == spec.ComponentPump && !anyPumpReady {
			ctx, cancel := context.WithTimeout(context.TODO(), time.Second*120)
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
		return err
	}

	p.booted = true

	succ := p.waitAllTidbUp()

	var monitorInfo *MonitorInfo
	if options.Monitor {
		var err error

		p.monitor, monitorInfo, err = p.bootMonitor(ctx, env)
		if err != nil {
			return err
		}

		p.ngmonitoring, err = p.bootNGMonitoring(ctx, env)
		if err != nil {
			return err
		}

		p.grafana, err = p.bootGrafana(ctx, env, monitorInfo)
		if err != nil {
			return err
		}
	}

	if len(succ) > 0 {
		// start TiFlash after at least one TiDB is up.
		var started []*instance.TiFlashInstance
		for _, flash := range p.tiflashs {
			if err := p.startInstance(ctx, flash); err != nil {
				fmt.Println(color.RedString("TiFlash %s failed to start: %s", flash.Addr(), err))
			} else {
				started = append(started, flash)
			}
		}
		p.tiflashs = started
		p.waitAllTiFlashUp()

		fmt.Println(color.GreenString("CLUSTER START SUCCESSFULLY, Enjoy it ^-^"))
		for _, dbAddr := range succ {
			ss := strings.Split(dbAddr, ":")
			connectMsg := "To connect TiDB: mysql --comments --host %s --port %s -u root -p (no password)"
			fmt.Println(color.GreenString(connectMsg, ss[0], ss[1]))
		}
	}

	if pdAddr := p.pds[0].Addr(); len(p.tidbs) > 0 && hasDashboard(pdAddr) {
		fmt.Println(color.GreenString("To view the dashboard: http://%s/dashboard", pdAddr))
	}

	var pdAddrs []string
	for _, pd := range p.pds {
		pdAddrs = append(pdAddrs, pd.Addr())
	}
	fmt.Println(color.GreenString("PD client endpoints: %v", pdAddrs))

	if monitorInfo != nil {
		p.updateMonitorTopology(spec.ComponentPrometheus, *monitorInfo)
	}

	dumpDSN(filepath.Join(p.dataDir, "dsn"), p.tidbs)

	go func() {
		// fmt.Printf("serve at :%d\n", p.port)
		err := p.listenAndServeHTTP()
		if err != nil {
			fmt.Printf("listenAndServeHTTP quit: %s\n", err)
		}
	}()

	logIfErr(p.renderSDFile())

	if g := p.grafana; g != nil {
		p.updateMonitorTopology(spec.ComponentGrafana, MonitorInfo{g.host, g.port, g.cmd.Path})
	}

	return nil
}

func (p *Playground) updateMonitorTopology(componentID string, info MonitorInfo) {
	info.IP = instance.AdvertiseHost(info.IP)
	fmt.Print(color.GreenString(
		"To view the %s: http://%s:%d\n",
		cases.Title(language.English).String(componentID),
		info.IP,
		info.Port,
	))
	if len(p.pds) == 0 {
		return
	}

	client, err := newEtcdClient(p.pds[0].Addr())
	if err == nil && client != nil {
		if promBinary, err := json.Marshal(info); err == nil {
			_, err = client.Put(context.TODO(), "/topology/"+componentID, string(promBinary))
			if err != nil {
				fmt.Println("Set the PD metrics storage failed")
			}
		}
	}
}

// Wait all instance quit and return the first non-nil err.
// including p8s & grafana
func (p *Playground) wait() error {
	err := p.instanceWaiter.Wait()
	if err != nil && atomic.LoadInt32(&p.curSig) == 0 {
		return err
	}

	return nil
}

func (p *Playground) terminate(sig syscall.Signal) {
	kill := func(pid int, wait func() error) {
		if sig != syscall.SIGINT {
			_ = syscall.Kill(pid, sig)
		}

		timer := time.AfterFunc(forceKillAfterDuration, func() {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		})

		_ = wait()
		timer.Stop()
	}

	for i := len(p.startedInstances); i > 0; i-- {
		inst := p.startedInstances[i-1]
		if sig == syscall.SIGKILL {
			fmt.Printf("Force %s(%d) to quit...\n", inst.Component(), inst.Pid())
		} else if atomic.LoadInt32(&p.curSig) == int32(sig) { // In case of double ctr+c
			fmt.Printf("Wait %s(%d) to quit...\n", inst.Component(), inst.Pid())
		}

		kill(inst.Pid(), inst.Wait)
	}

	if p.monitor != nil {
		kill(p.monitor.cmd.Process.Pid, p.monitor.wait)
	}

	if p.ngmonitoring != nil {
		kill(p.ngmonitoring.cmd.Process.Pid, p.ngmonitoring.wait)
	}

	if p.grafana != nil {
		kill(p.grafana.cmd.Process.Pid, p.grafana.wait)
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
		return err
	}

	return nil
}

// return not error iff the Cmd is started successfully.
// user must and can safely wait the Cmd
func (p *Playground) bootMonitor(ctx context.Context, env *environment.Environment) (*monitor, *MonitorInfo, error) {
	options := p.bootOptions
	monitorInfo := &MonitorInfo{}

	dataDir := p.dataDir
	promDir := filepath.Join(dataDir, "prometheus")

	monitor, err := newMonitor(ctx, options.Version, options.Host, promDir)
	if err != nil {
		return nil, nil, err
	}

	monitorInfo.IP = instance.AdvertiseHost(options.Host)
	monitorInfo.BinaryPath = promDir
	monitorInfo.Port = monitor.port

	// start the monitor cmd.
	log, err := os.OpenFile(filepath.Join(promDir, "prom.log"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, os.ModePerm)
	if err != nil {
		return nil, nil, errors.AddStack(err)
	}
	defer log.Close()

	monitor.cmd.Stderr = log
	monitor.cmd.Stdout = os.Stdout

	if err := monitor.cmd.Start(); err != nil {
		return nil, nil, err
	}

	p.instanceWaiter.Go(func() error {
		err := monitor.wait()
		if err != nil && atomic.LoadInt32(&p.curSig) == 0 {
			fmt.Printf("Prometheus quit: %v\n", err)
		} else {
			fmt.Println("prometheus quit")
		}
		return err
	})

	return monitor, monitorInfo, nil
}

// return not error iff the Cmd is started successfully.
// user must and can safely wait the Cmd
func (p *Playground) bootNGMonitoring(ctx context.Context, env *environment.Environment) (*ngMonitoring, error) {
	options := p.bootOptions

	dataDir := p.dataDir
	promDir := filepath.Join(dataDir, "prometheus")

	ngm, err := newNGMonitoring(ctx, options.Version, options.Host, promDir, p.pds)
	if err != nil {
		return nil, err
	}

	// ng-monitoring only exists when tidb >= 5.3.0
	_, err = os.Stat(ngm.cmd.Path)
	if os.IsNotExist(err) {
		return nil, nil
	}

	if err := ngm.cmd.Start(); err != nil {
		return nil, err
	}

	p.instanceWaiter.Go(func() error {
		err := ngm.wait()
		if err != nil && atomic.LoadInt32(&p.curSig) == 0 {
			fmt.Printf("ng-monitoring quit: %v\n", err)
		} else {
			fmt.Println("ng-monitoring quit")
		}
		return err
	})

	return ngm, nil
}

// return not error iff the Cmd is started successfully.
func (p *Playground) bootGrafana(ctx context.Context, env *environment.Environment, monitorInfo *MonitorInfo) (*grafana, error) {
	// set up grafana
	options := p.bootOptions
	if err := installIfMissing("grafana", options.Version); err != nil {
		return nil, err
	}
	installPath, err := env.Profile().ComponentInstalledPath("grafana", utils.Version(options.Version))
	if err != nil {
		return nil, err
	}

	dataDir := p.dataDir
	grafanaDir := filepath.Join(dataDir, "grafana")

	cmd := exec.Command("cp", "-Rfp", installPath, grafanaDir)
	err = cmd.Run()
	if err != nil {
		return nil, errors.AddStack(err)
	}

	dashboardDir := filepath.Join(grafanaDir, "dashboards")
	err = os.MkdirAll(dashboardDir, 0755)
	if err != nil {
		return nil, errors.AddStack(err)
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
		return nil, err
	}

	err = replaceDatasource(dashboardDir, clusterName)
	if err != nil {
		return nil, err
	}

	grafana := newGrafana(options.Version, options.Host)
	// fmt.Println("Start Grafana instance...")
	err = grafana.start(ctx, grafanaDir, fmt.Sprintf("http://%s:%d", monitorInfo.IP, monitorInfo.Port))
	if err != nil {
		return nil, err
	}

	p.instanceWaiter.Go(func() error {
		err := grafana.wait()
		if err != nil && atomic.LoadInt32(&p.curSig) == 0 {
			fmt.Printf("Grafana quit: %v\n", err)
		} else {
			fmt.Println("Grafana quit")
		}
		return err
	})

	return grafana, nil
}

func logIfErr(err error) {
	if err != nil {
		fmt.Println(err)
	}
}
