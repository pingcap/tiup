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
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/instance"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/environment"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/tui/colorstr"
	"github.com/pingcap/tiup/pkg/tui/progress"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
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
	tsos             []*instance.PDInstance
	schedulings      []*instance.PDInstance
	tikvs            []*instance.TiKVInstance
	tidbs            []*instance.TiDBInstance
	tiflashs         []*instance.TiFlashInstance
	tiproxys         []*instance.TiProxy
	ticdcs           []*instance.TiCDC
	tikvCdcs         []*instance.TiKVCDC
	pumps            []*instance.Pump
	drainers         []*instance.Drainer
	dmMasters        []*instance.DMMaster
	dmWorkers        []*instance.DMWorker
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
	// TODO add more info.
	td := utils.NewTableDisplayer(r, []string{"Pid", "Role", "Uptime"})

	err = p.WalkInstances(func(componentID string, ins instance.Instance) error {
		td.AddRow(strconv.Itoa(ins.Pid()), componentID, ins.Uptime())
		return nil
	})

	if err != nil {
		return err
	}

	td.Display()
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

	return api.NewBinlogClient(addrs, 5*time.Second, nil)
}

func (p *Playground) dmMasterClient() *api.DMMasterClient {
	var addrs []string
	for _, inst := range p.dmMasters {
		addrs = append(addrs, inst.Addr())
	}

	return api.NewDMMasterClient(addrs, 5*time.Second, nil)
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
	case spec.ComponentTSO:
		for i := 0; i < len(p.tsos); i++ {
			if p.tsos[i].Pid() == pid {
				p.tsos = append(p.tsos[:i], p.tsos[i+1:]...)
			}
		}
	case spec.ComponentScheduling:
		for i := 0; i < len(p.schedulings); i++ {
			if p.schedulings[i].Pid() == pid {
				p.schedulings = append(p.schedulings[:i], p.schedulings[i+1:]...)
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
	case spec.ComponentTiProxy:
		for i := 0; i < len(p.tiproxys); i++ {
			if p.tiproxys[i].Pid() == pid {
				p.tiproxys = append(p.tiproxys[:i], p.tiproxys[i+1:]...)
			}
		}
	case spec.ComponentTiKVCDC:
		for i := 0; i < len(p.tikvCdcs); i++ {
			if p.tikvCdcs[i].Pid() == pid {
				p.tikvCdcs = append(p.tikvCdcs[:i], p.tikvCdcs[i+1:]...)
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
				fmt.Fprintf(w, "TiFlash will be stop when tombstone\n")
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
	case spec.ComponentDMWorker:
		if err := p.handleScaleInDMWorker(pid); err != nil {
			return err
		}
	case spec.ComponentDMMaster:
		if err := p.handleScaleInDMMaster(pid); err != nil {
			return err
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

func (p *Playground) handleScaleInDMWorker(pid int) error {
	for i := 0; i < len(p.dmWorkers); i++ {
		if p.dmWorkers[i].Pid() == pid {
			inst := p.dmWorkers[i]

			c := p.dmMasterClient()
			if err := c.OfflineWorker(inst.Name(), nil); err != nil {
				return err
			}
			p.dmWorkers = append(p.dmWorkers[:i], p.dmWorkers[i+1:]...)
			return nil
		}
	}
	return nil
}

func (p *Playground) handleScaleInDMMaster(pid int) error {
	for i := 0; i < len(p.dmMasters); i++ {
		if p.dmMasters[i].Pid() == pid {
			inst := p.dmMasters[i]

			c := p.dmMasterClient()
			if err := c.OfflineMaster(inst.Name(), nil); err != nil {
				return err
			}
			p.dmMasters = append(p.dmMasters[:i], p.dmMasters[i+1:]...)
			return nil
		}
	}
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
	case spec.ComponentTSO:
		return p.sanitizeConfig(p.bootOptions.TSO, cfg)
	case spec.ComponentScheduling:
		return p.sanitizeConfig(p.bootOptions.Scheduling, cfg)
	case spec.ComponentTiKV:
		return p.sanitizeConfig(p.bootOptions.TiKV, cfg)
	case spec.ComponentTiDB:
		return p.sanitizeConfig(p.bootOptions.TiDB, cfg)
	case spec.ComponentTiFlash:
		return p.sanitizeConfig(p.bootOptions.TiFlash, cfg)
	case spec.ComponentCDC:
		return p.sanitizeConfig(p.bootOptions.TiCDC, cfg)
	case spec.ComponentTiKVCDC:
		return p.sanitizeConfig(p.bootOptions.TiKVCDC, cfg)
	case spec.ComponentPump:
		return p.sanitizeConfig(p.bootOptions.Pump, cfg)
	case spec.ComponentDrainer:
		return p.sanitizeConfig(p.bootOptions.Drainer, cfg)
	case spec.ComponentTiProxy:
		return p.sanitizeConfig(p.bootOptions.TiProxy, cfg)
	case spec.ComponentDMMaster:
		return p.sanitizeConfig(p.bootOptions.DMMaster, cfg)
	case spec.ComponentDMWorker:
		return p.sanitizeConfig(p.bootOptions.DMWorker, cfg)
	default:
		return fmt.Errorf("unknown %s in sanitizeConfig", cid)
	}
}

func (p *Playground) startInstance(ctx context.Context, inst instance.Instance) error {
	var version utils.Version
	var err error
	boundVersion := p.bindVersion(inst.Component(), p.bootOptions.Version)
	component := inst.Component()
	if component == "tso" || component == "scheduling" {
		component = string(instance.PDRoleNormal)
	}
	if version, err = environment.GlobalEnv().V1Repository().ResolveComponentVersion(component, boundVersion); err != nil {
		return err
	}

	if err := inst.PrepareBinary(component, inst.Component(), version); err != nil {
		return err
	}

	if err = inst.Start(ctx); err != nil {
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
	// TODO: Support scale-out in CSE mode
	inst, err := p.addInstance(cmd.ComponentID, instance.PDRoleNormal, instance.TiFlashRoleNormal, cmd.Config)
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

	mysql := mysqlCommand()
	if cmd.ComponentID == "tidb" {
		addr := p.tidbs[len(p.tidbs)-1].Addr()
		if checkDB(addr, cmd.UpTimeout) {
			ss := strings.Split(addr, ":")
			connectMsg := "To connect new added TiDB: %s --host %s --port %s -u root -p (no password)"
			fmt.Println(color.GreenString(connectMsg, mysql, ss[0], ss[1]))
			fmt.Fprintln(w, color.GreenString(connectMsg, mysql, ss[0], ss[1]))
		}
	}
	if cmd.ComponentID == "tiproxy" {
		addr := p.tiproxys[len(p.tidbs)-1].Addr()
		if checkDB(addr, cmd.UpTimeout) {
			ss := strings.Split(addr, ":")
			connectMsg := "To connect to the newly added TiProxy: %s --host %s --port %s -u root -p (no password)"
			fmt.Println(color.GreenString(connectMsg, mysql, ss[0], ss[1]))
			fmt.Fprintln(w, color.GreenString(connectMsg, mysql, ss[0], ss[1]))
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

	// Mapping command line component id to internal spec component id.
	if cmd.ComponentID == "ticdc" {
		cmd.ComponentID = spec.ComponentCDC
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
	for _, ins := range p.tsos {
		err := fn(spec.ComponentTSO, ins)
		if err != nil {
			return err
		}
	}
	for _, ins := range p.schedulings {
		err := fn(spec.ComponentScheduling, ins)
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

	for _, ins := range p.tiproxys {
		err := fn(spec.ComponentTiProxy, ins)
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

	for _, ins := range p.tikvCdcs {
		err := fn(spec.ComponentTiKVCDC, ins)
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

	for _, ins := range p.dmMasters {
		err := fn(spec.ComponentDMMaster, ins)
		if err != nil {
			return err
		}
	}

	for _, ins := range p.dmWorkers {
		err := fn(spec.ComponentDMWorker, ins)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Playground) enableBinlog() bool {
	return p.bootOptions.Pump.Num > 0
}

func (p *Playground) addInstance(componentID string, pdRole instance.PDRole, tiflashRole instance.TiFlashRole, cfg instance.Config) (ins instance.Instance, err error) {
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
	if componentID == string(instance.PDRoleNormal) && (pdRole != instance.PDRoleNormal && pdRole != instance.PDRoleAPI) {
		id = p.allocID(string(pdRole))
		dir = filepath.Join(dataDir, fmt.Sprintf("%s-%d", pdRole, id))
	}
	if err = utils.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	// look more like listen ip?
	host := p.bootOptions.Host
	if cfg.Host != "" {
		host = cfg.Host
	}

	switch componentID {
	case spec.ComponentPD:
		inst := instance.NewPDInstance(pdRole, cfg.BinPath, dir, host, cfg.ConfigPath, options.PortOffset, id, p.pds, cfg.Port, p.bootOptions.Mode, p.bootOptions.TiKV.Num == 1)
		ins = inst
		if pdRole == instance.PDRoleNormal || pdRole == instance.PDRoleAPI {
			if p.booted {
				inst.Join(p.pds)
				p.pds = append(p.pds, inst)
			} else {
				p.pds = append(p.pds, inst)
				for _, pd := range p.pds {
					pd.InitCluster(p.pds)
				}
			}
		} else if pdRole == instance.PDRoleTSO {
			p.tsos = append(p.tsos, inst)
		} else if pdRole == instance.PDRoleScheduling {
			p.schedulings = append(p.schedulings, inst)
		}
	case spec.ComponentTSO:
		inst := instance.NewPDInstance(instance.PDRoleTSO, cfg.BinPath, dir, host, cfg.ConfigPath, options.PortOffset, id, p.pds, cfg.Port, p.bootOptions.Mode, p.bootOptions.TiKV.Num == 1)
		ins = inst
		p.tsos = append(p.tsos, inst)
	case spec.ComponentScheduling:
		inst := instance.NewPDInstance(instance.PDRoleScheduling, cfg.BinPath, dir, host, cfg.ConfigPath, options.PortOffset, id, p.pds, cfg.Port, p.bootOptions.Mode, p.bootOptions.TiKV.Num == 1)
		ins = inst
		p.schedulings = append(p.schedulings, inst)
	case spec.ComponentTiDB:
		inst := instance.NewTiDBInstance(cfg.BinPath, dir, host, cfg.ConfigPath, options.PortOffset, id, cfg.Port, p.pds, dataDir, p.enableBinlog(), p.bootOptions.Mode)
		ins = inst
		p.tidbs = append(p.tidbs, inst)
	case spec.ComponentTiKV:
		inst := instance.NewTiKVInstance(cfg.BinPath, dir, host, cfg.ConfigPath, options.PortOffset, id, cfg.Port, p.pds, p.tsos, p.bootOptions.Mode, p.bootOptions.CSEOpts, p.bootOptions.PDMode == "ms")
		ins = inst
		p.tikvs = append(p.tikvs, inst)
	case spec.ComponentTiFlash:
		inst := instance.NewTiFlashInstance(p.bootOptions.Mode, tiflashRole, p.bootOptions.CSEOpts, cfg.BinPath, dir, host, cfg.ConfigPath, options.PortOffset, id, p.pds, p.tidbs, cfg.Version)
		ins = inst
		p.tiflashs = append(p.tiflashs, inst)
	case spec.ComponentTiProxy:
		if err := instance.GenTiProxySessionCerts(dataDir); err != nil {
			return nil, err
		}
		inst := instance.NewTiProxy(cfg.BinPath, dir, host, cfg.ConfigPath, options.PortOffset, id, cfg.Port, p.pds)
		ins = inst
		p.tiproxys = append(p.tiproxys, inst)
	case spec.ComponentCDC:
		inst := instance.NewTiCDC(cfg.BinPath, dir, host, cfg.ConfigPath, options.PortOffset, id, cfg.Port, p.pds)
		ins = inst
		p.ticdcs = append(p.ticdcs, inst)
	case spec.ComponentTiKVCDC:
		inst := instance.NewTiKVCDC(cfg.BinPath, dir, host, cfg.ConfigPath, options.PortOffset, id, p.pds)
		ins = inst
		p.tikvCdcs = append(p.tikvCdcs, inst)
	case spec.ComponentPump:
		inst := instance.NewPump(cfg.BinPath, dir, host, cfg.ConfigPath, options.PortOffset, id, p.pds)
		ins = inst
		p.pumps = append(p.pumps, inst)
	case spec.ComponentDrainer:
		inst := instance.NewDrainer(cfg.BinPath, dir, host, cfg.ConfigPath, options.PortOffset, id, p.pds)
		ins = inst
		p.drainers = append(p.drainers, inst)
	case spec.ComponentDMMaster:
		inst := instance.NewDMMaster(cfg.BinPath, dir, host, cfg.ConfigPath, options.PortOffset, id, cfg.Port)
		ins = inst
		p.dmMasters = append(p.dmMasters, inst)
		for _, master := range p.dmMasters {
			master.SetInitEndpoints(p.dmMasters)
		}
	case spec.ComponentDMWorker:
		inst := instance.NewDMWorker(cfg.BinPath, dir, host, cfg.ConfigPath, options.PortOffset, id, cfg.Port, p.dmMasters)
		ins = inst
		p.dmWorkers = append(p.dmWorkers, inst)
	default:
		return nil, errors.Errorf("unknown component: %s", componentID)
	}

	return
}

func (p *Playground) waitAllDBUp() ([]string, []string) {
	var tidbSucc []string
	var tiproxySucc []string
	if len(p.tidbs) > 0 {
		var wg sync.WaitGroup
		var tidbMu, tiproxyMu sync.Mutex
		var bars *progress.MultiBar
		if len(p.tiproxys) > 0 {
			bars = progress.NewMultiBar(colorstr.Sprintf("[dark_gray]Waiting for tidb and tiproxy instances ready"))
		} else {
			bars = progress.NewMultiBar(colorstr.Sprintf("[dark_gray]Waiting for tidb instances ready"))
		}
		for _, db := range p.tidbs {
			wg.Add(1)
			prefix := "- TiDB: " + db.Addr()
			bar := bars.AddBar(prefix)
			go func(dbInst *instance.TiDBInstance) {
				defer wg.Done()
				if s := checkDB(dbInst.Addr(), options.TiDB.UpTimeout); s {
					{
						tidbMu.Lock()
						tidbSucc = append(tidbSucc, dbInst.Addr())
						tidbMu.Unlock()
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
		for _, db := range p.tiproxys {
			wg.Add(1)
			prefix := "- TiProxy: " + db.Addr()
			bar := bars.AddBar(prefix)
			go func(dbInst *instance.TiProxy) {
				defer wg.Done()
				if s := checkDB(dbInst.Addr(), options.TiProxy.UpTimeout); s {
					{
						tiproxyMu.Lock()
						tiproxySucc = append(tiproxySucc, dbInst.Addr())
						tiproxyMu.Unlock()
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
	return tidbSucc, tiproxySucc
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
		bars := progress.NewMultiBar(colorstr.Sprintf("[dark_gray]Waiting for tiflash instances ready"))
		for _, flash := range p.tiflashs {
			wg.Add(1)

			tiflashKindName := "TiFlash"
			if flash.Role == instance.TiFlashRoleDisaggCompute {
				tiflashKindName = "TiFlash (CN)"
			} else if flash.Role == instance.TiFlashRoleDisaggWrite {
				tiflashKindName = "TiFlash (WN)"
			}

			prefix := fmt.Sprintf("- %s: %s", tiflashKindName, flash.Addr())
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

func (p *Playground) waitAllDMMasterUp() {
	if len(p.dmMasters) > 0 {
		var wg sync.WaitGroup
		bars := progress.NewMultiBar(colorstr.Sprintf("[dark_gray]Waiting for dm-master instances ready"))
		for _, master := range p.dmMasters {
			wg.Add(1)
			prefix := master.Addr()
			bar := bars.AddBar(prefix)
			go func(masterInst *instance.DMMaster) {
				defer wg.Done()
				displayResult := &progress.DisplayProps{
					Prefix: prefix,
				}
				if cmd := masterInst.Cmd(); cmd == nil {
					displayResult.Mode = progress.ModeError
					displayResult.Suffix = "initialize command failed"
				} else if state := cmd.ProcessState; state != nil && state.Exited() {
					displayResult.Mode = progress.ModeError
					displayResult.Suffix = fmt.Sprintf("process exited with code: %d", state.ExitCode())
				} else if s := checkDMMasterStatus(p.dmMasterClient(), masterInst.Name(), options.DMMaster.UpTimeout); !s {
					displayResult.Mode = progress.ModeError
					displayResult.Suffix = "failed to up after timeout"
				} else {
					displayResult.Mode = progress.ModeDone
				}
				bar.UpdateDisplay(displayResult)
			}(master)
		}
		bars.StartRenderLoop()
		wg.Wait()
		bars.StopRenderLoop()
	}
}

func (p *Playground) bindVersion(comp string, version string) (bindVersion string) {
	bindVersion = version
	switch comp {
	case spec.ComponentTiKVCDC:
		bindVersion = p.bootOptions.TiKVCDC.Version
	case spec.ComponentTiProxy:
		bindVersion = p.bootOptions.TiProxy.Version
	default:
	}
	return
}

//revive:disable:cognitive-complexity
//revive:disable:error-strings
func (p *Playground) bootCluster(ctx context.Context, env *environment.Environment, options *BootOptions) error {
	for _, cfg := range []*instance.Config{
		&options.PD,
		&options.TSO,
		&options.Scheduling,
		&options.TiProxy,
		&options.TiDB,
		&options.TiKV,
		&options.TiFlash,
		&options.TiFlashCompute,
		&options.TiFlashWrite,
		&options.Pump,
		&options.Drainer,
		&options.TiKVCDC,
		&options.DMMaster,
		&options.DMWorker,
	} {
		path, err := getAbsolutePath(cfg.ConfigPath)
		if err != nil {
			return errors.Annotatef(err, "cannot eval absolute directory: %s", cfg.ConfigPath)
		}
		cfg.ConfigPath = path
	}

	p.bootOptions = options

	// All others components depend on the pd except dm, we just ensure the pd count must be great than 0
	if options.PDMode != "ms" && options.PD.Num < 1 && options.DMMaster.Num < 1 {
		return fmt.Errorf("all components count must be great than 0 (pd=%v)", options.PD.Num)
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

	type InstancePair struct {
		comp        string
		pdRole      instance.PDRole
		tiflashRole instance.TiFlashRole
		instance.Config
	}

	instances := []InstancePair{
		{spec.ComponentTiProxy, "", "", options.TiProxy},
		{spec.ComponentTiKV, "", "", options.TiKV},
		{spec.ComponentPump, "", "", options.Pump},
		{spec.ComponentTiDB, "", "", options.TiDB},
		{spec.ComponentCDC, "", "", options.TiCDC},
		{spec.ComponentTiKVCDC, "", "", options.TiKVCDC},
		{spec.ComponentDrainer, "", "", options.Drainer},
		{spec.ComponentDMMaster, "", "", options.DMMaster},
		{spec.ComponentDMWorker, "", "", options.DMWorker},
	}

	if options.Mode == "tidb" {
		instances = append(instances,
			InstancePair{spec.ComponentTiFlash, instance.PDRoleNormal, instance.TiFlashRoleNormal, options.TiFlash},
		)
	} else if options.Mode == "tidb-cse" || options.Mode == "tiflash-disagg" {
		if !tidbver.TiFlashPlaygroundNewStartMode(options.Version) {
			// For simplicity, currently we only implemented disagg mode when TiFlash can run without config.
			return fmt.Errorf("TiUP playground only supports CSE/Disagg mode for TiDB cluster >= v7.1.0 (or nightly)")
		}

		if !strings.HasPrefix(options.CSEOpts.S3Endpoint, "https://") && !strings.HasPrefix(options.CSEOpts.S3Endpoint, "http://") {
			return fmt.Errorf("CSE/Disagg mode requires S3 endpoint to start with http:// or https://")
		}

		isSecure := strings.HasPrefix(options.CSEOpts.S3Endpoint, "https://")
		rawEndpoint := strings.TrimPrefix(options.CSEOpts.S3Endpoint, "https://")
		rawEndpoint = strings.TrimPrefix(rawEndpoint, "http://")

		// Currently we always assign region=local. Other regions are not supported.
		if strings.Contains(rawEndpoint, "amazonaws.com") {
			return fmt.Errorf("Currently TiUP playground CSE/Disagg mode only supports local S3 (like minio). S3 on AWS Regions are not supported. Contributions are welcome!")
		}

		// Preflight check whether specified object storage is available.
		s3Client, err := minio.New(rawEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(options.CSEOpts.AccessKey, options.CSEOpts.SecretKey, ""),
			Secure: isSecure,
		})
		if err != nil {
			return errors.Annotate(err, "CSE/Disagg mode preflight check failed")
		}

		ctxCheck, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		bucketExists, err := s3Client.BucketExists(ctxCheck, options.CSEOpts.Bucket)
		if err != nil {
			return errors.Annotate(err, "CSE/Disagg mode preflight check failed")
		}

		if !bucketExists {
			// Try to create bucket.
			err := s3Client.MakeBucket(ctxCheck, options.CSEOpts.Bucket, minio.MakeBucketOptions{})
			if err != nil {
				return fmt.Errorf("CSE/Disagg mode preflight check failed: Bucket %s doesn't exist and fail to create automatically (your bucket name may be invalid?)", options.CSEOpts.Bucket)
			}
		}

		instances = append(
			instances,
			InstancePair{spec.ComponentTiFlash, instance.PDRoleNormal, instance.TiFlashRoleDisaggWrite, options.TiFlashWrite},
			InstancePair{spec.ComponentTiFlash, instance.PDRoleNormal, instance.TiFlashRoleDisaggCompute, options.TiFlashCompute},
		)
	}

	if options.PDMode == "pd" {
		instances = append([]InstancePair{{spec.ComponentPD, instance.PDRoleNormal, instance.TiFlashRoleNormal, options.PD}},
			instances...,
		)
	} else if options.PDMode == "ms" {
		if !tidbver.PDSupportMicroservices(options.Version) {
			return fmt.Errorf("PD cluster doesn't support microservices mode in version %s", options.Version)
		}
		instances = append([]InstancePair{
			{spec.ComponentPD, instance.PDRoleAPI, instance.TiFlashRoleNormal, options.PD},
			{spec.ComponentPD, instance.PDRoleTSO, instance.TiFlashRoleNormal, options.TSO},
			{spec.ComponentPD, instance.PDRoleScheduling, instance.TiFlashRoleNormal, options.Scheduling}},
			instances...,
		)
	}

	for _, inst := range instances {
		for i := 0; i < inst.Num; i++ {
			_, err := p.addInstance(inst.comp, inst.pdRole, inst.tiflashRole, inst.Config)
			if err != nil {
				return err
			}
		}
	}

	anyPumpReady := false
	allDMMasterReady := false
	// Start all instance except tiflash.
	err := p.WalkInstances(func(cid string, ins instance.Instance) error {
		if cid == spec.ComponentTiFlash {
			return nil
		}
		// wait dm-master up before dm-worker
		if cid == spec.ComponentDMWorker && !allDMMasterReady {
			p.waitAllDMMasterUp()
			allDMMasterReady = true
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

	tidbSucc, tiproxySucc := p.waitAllDBUp()

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

	colorCmd := color.New(color.FgHiCyan, color.Bold)

	if len(tidbSucc) > 0 {
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

		fmt.Println()
		color.New(color.FgGreen, color.Bold).Println("🎉 TiDB Playground Cluster is started, enjoy!")

		if deleteWhenExit {
			fmt.Println()
			colorstr.Printf("[yellow][bold]Warning[reset][bold]: cluster data will be destroyed after exit. To persist data after exit, specify [tiup_command]--tag <name>[reset].\n")
		}

		fmt.Println()
		mysql := mysqlCommand()
		for _, dbAddr := range tidbSucc {
			ss := strings.Split(dbAddr, ":")
			fmt.Printf("Connect TiDB:    ")
			colorCmd.Printf("%s --host %s --port %s -u root\n", mysql, ss[0], ss[1])
		}
		for _, dbAddr := range tiproxySucc {
			ss := strings.Split(dbAddr, ":")
			fmt.Printf("Connect TiProxy: ")
			colorCmd.Printf("%s --host %s --port %s -u root\n", mysql, ss[0], ss[1])
		}
	}

	if len(p.dmMasters) > 0 {
		fmt.Printf("Connect DM:      ")
		endpoints := make([]string, 0, len(p.dmMasters))
		for _, dmMaster := range p.dmMasters {
			endpoints = append(endpoints, dmMaster.Addr())
		}
		colorCmd.Printf("tiup dmctl --master-addr %s\n", strings.Join(endpoints, ","))
	}

	if len(p.pds) > 0 {
		if pdAddr := p.pds[0].Addr(); len(p.tidbs) > 0 && hasDashboard(pdAddr) {
			fmt.Printf("TiDB Dashboard:  ")
			colorCmd.Printf("http://%s/dashboard\n", pdAddr)
		}
	}

	if p.bootOptions.Mode == "tikv-slim" {
		if p.bootOptions.PDMode == "ms" {
			var (
				tsoAddr        []string
				apiAddr        []string
				schedulingAddr []string
			)
			for _, api := range p.pds {
				apiAddr = append(apiAddr, api.Addr())
			}
			for _, tso := range p.tsos {
				tsoAddr = append(tsoAddr, tso.Addr())
			}
			for _, scheduling := range p.schedulings {
				schedulingAddr = append(schedulingAddr, scheduling.Addr())
			}

			fmt.Printf("PD API Endpoints:   ")
			colorCmd.Printf("%s\n", strings.Join(apiAddr, ","))
			fmt.Printf("PD TSO Endpoints:   ")
			colorCmd.Printf("%s\n", strings.Join(tsoAddr, ","))
			fmt.Printf("PD Scheduling Endpoints:   ")
			colorCmd.Printf("%s\n", strings.Join(schedulingAddr, ","))
		} else {
			var pdAddrs []string
			for _, pd := range p.pds {
				pdAddrs = append(pdAddrs, pd.Addr())
			}
			fmt.Printf("PD Endpoints:   ")
			colorCmd.Printf("%s\n", strings.Join(pdAddrs, ","))
		}
	}

	if monitorInfo != nil {
		p.updateMonitorTopology(spec.ComponentPrometheus, *monitorInfo)
	}

	dumpDSN(filepath.Join(p.dataDir, "dsn"), p.tidbs, p.tiproxys)

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
		fmt.Printf("Grafana:         ")
		colorCmd.Printf("http://%s\n", utils.JoinHostPort(g.host, g.port))
	}

	return nil
}

func (p *Playground) updateMonitorTopology(componentID string, info MonitorInfo) {
	info.IP = instance.AdvertiseHost(info.IP)
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
	kill := func(name string, pid int, wait func() error) {
		if sig == syscall.SIGKILL {
			colorstr.Printf("[dark_gray]Force %s(%d) to quit...\n", name, pid)
		} else if atomic.LoadInt32(&p.curSig) == int32(sig) { // In case of double ctr+c
			colorstr.Printf("[dark_gray]Wait %s(%d) to quit...\n", name, pid)
		}

		_ = syscall.Kill(pid, sig)
		timer := time.AfterFunc(forceKillAfterDuration, func() {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		})

		_ = wait()
		timer.Stop()
	}

	if p.monitor != nil && p.monitor.cmd != nil && p.monitor.cmd.Process != nil {
		go kill("prometheus", p.monitor.cmd.Process.Pid, p.monitor.wait)
	}

	if p.ngmonitoring != nil && p.ngmonitoring.cmd != nil && p.ngmonitoring.cmd.Process != nil {
		go kill("ng-monitoring", p.ngmonitoring.cmd.Process.Pid, p.ngmonitoring.wait)
	}

	if p.grafana != nil && p.grafana.cmd != nil && p.grafana.cmd.Process != nil {
		go kill("grafana", p.grafana.cmd.Process.Pid, p.grafana.wait)
	}

	for _, inst := range p.dmWorkers {
		if inst.Process != nil && inst.Process.Cmd() != nil && inst.Process.Cmd().Process != nil {
			kill(inst.Component(), inst.Pid(), inst.Wait)
		}
	}

	for _, inst := range p.dmMasters {
		if inst.Process != nil && inst.Process.Cmd() != nil && inst.Process.Cmd().Process != nil {
			kill(inst.Component(), inst.Pid(), inst.Wait)
		}
	}

	for _, inst := range p.tiflashs {
		if inst.Process != nil && inst.Process.Cmd() != nil && inst.Process.Cmd().Process != nil {
			kill(inst.Component(), inst.Pid(), inst.Wait)
		}
	}
	for _, inst := range p.ticdcs {
		if inst.Process != nil && inst.Process.Cmd() != nil && inst.Process.Cmd().Process != nil {
			kill(inst.Component(), inst.Pid(), inst.Wait)
		}
	}
	for _, inst := range p.tikvCdcs {
		if inst.Process != nil && inst.Process.Cmd() != nil && inst.Process.Cmd().Process != nil {
			kill(inst.Component(), inst.Pid(), inst.Wait)
		}
	}
	for _, inst := range p.drainers {
		if inst.Process != nil && inst.Process.Cmd() != nil && inst.Process.Cmd().Process != nil {
			kill(inst.Component(), inst.Pid(), inst.Wait)
		}
	}
	// tidb must exit earlier then pd
	for _, inst := range p.tidbs {
		if inst.Process != nil && inst.Process.Cmd() != nil && inst.Process.Cmd().Process != nil {
			kill(inst.Component(), inst.Pid(), inst.Wait)
		}
	}
	for _, inst := range p.pumps {
		if inst.Process != nil && inst.Process.Cmd() != nil && inst.Process.Cmd().Process != nil {
			kill(inst.Component(), inst.Pid(), inst.Wait)
		}
	}
	for _, inst := range p.tikvs {
		if inst.Process != nil && inst.Process.Cmd() != nil && inst.Process.Cmd().Process != nil {
			kill(inst.Component(), inst.Pid(), inst.Wait)
		}
	}
	for _, inst := range p.pds {
		if inst.Process != nil && inst.Process.Cmd() != nil && inst.Process.Cmd().Process != nil {
			kill(inst.Component(), inst.Pid(), inst.Wait)
		}
	}
	for _, inst := range p.tsos {
		if inst.Process != nil && inst.Process.Cmd() != nil && inst.Process.Cmd().Process != nil {
			kill(inst.Component(), inst.Pid(), inst.Wait)
		}
	}
	for _, inst := range p.schedulings {
		if inst.Process != nil && inst.Process.Cmd() != nil && inst.Process.Cmd().Process != nil {
			kill(inst.Component(), inst.Pid(), inst.Wait)
		}
	}
	for _, inst := range p.tiproxys {
		if inst.Process != nil && inst.Process.Cmd() != nil && inst.Process.Cmd().Process != nil {
			kill(inst.Component(), inst.Pid(), inst.Wait)
		}
	}
}

func (p *Playground) renderSDFile() error {
	// we not start monitor at all.
	if p.monitor == nil {
		return nil
	}

	cid2targets := make(map[string]instance.MetricAddr)

	_ = p.WalkInstances(func(cid string, inst instance.Instance) error {
		v := inst.MetricAddr()
		t, ok := cid2targets[inst.Component()]
		if ok {
			v.Targets = append(v.Targets, t.Targets...)
		}
		cid2targets[inst.Component()] = v
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

	monitor, err := newMonitor(ctx, options.Version, options.Host, promDir, options.PortOffset)
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

	ngm, err := newNGMonitoring(ctx, options.Version, options.Host, promDir, options.PortOffset, p.pds)
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
	err = utils.MkdirAll(dashboardDir, 0755)
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

	grafana := newGrafana(options.Version, options.Host, options.GrafanaPort)
	// fmt.Println("Start Grafana instance...")
	err = grafana.start(ctx, grafanaDir, options.PortOffset, "http://"+utils.JoinHostPort(monitorInfo.IP, monitorInfo.Port))
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

// Check the MySQL Client version
//
// Since v8.1.0 `--comments` is the default, so we don't need to specify it.
// Without `--comments` the MySQL client strips TiDB specific comments
// like `/*T![clustered_index] CLUSTERED */`
//
// This returns `mysql --comments` for older versions or in case we failed to check
// the version for any reason as `mysql --comments` is the safe option.
// For newer MySQL versions it returns just `mysql`.
//
// For MariaDB versions of the MySQL Client it is expected to return `mysql --comments`.
func mysqlCommand() (cmd string) {
	cmd = "mysql --comments"
	mysqlVerOutput, err := exec.Command("mysql", "--version").Output()
	if err != nil {
		return
	}
	vMaj, vMin, _, err := parseMysqlVersion(string(mysqlVerOutput))
	if err == nil {
		// MySQL Client 8.1.0 and newer
		if vMaj == 8 && vMin >= 1 {
			return "mysql"
		}
		// MySQL Client 9.x.x. Note that 10.x is likely to be MariaDB, so not using >= here.
		if vMaj == 9 {
			return "mysql"
		}
	}
	return
}

// parseMysqlVersion parses the output from `mysql --version` that is in `versionOutput`
// and returns the major, minor and patch version.
//
// New format example: `mysql  Ver 8.2.0 for Linux on x86_64 (MySQL Community Server - GPL)`
// Old format example: `mysql  Ver 14.14 Distrib 5.7.36, for linux-glibc2.12 (x86_64) using  EditLine wrapper`
// MariaDB 11.2 format: `/usr/bin/mysql from 11.2.2-MariaDB, client 15.2 for linux-systemd (x86_64) using readline 5.1`
//
// Note that MariaDB has `bin/mysql` (deprecated) and `bin/mariadb`. This is to parse the version from `bin/mysql`.
// As TiDB is a MySQL compatible database we recommend `bin/mysql` from MySQL.
// If we ever want to auto-detect other clients like `bin/mariadb`, `bin/mysqlsh`, `bin/mycli`, etc then
// each of them needs their own version detection and adjust for the right commandline options.
func parseMysqlVersion(versionOutput string) (vMaj int, vMin int, vPatch int, err error) {
	mysqlVerRegexp := regexp.MustCompile(`(Ver|Distrib|from) ([0-9]+)\.([0-9]+)\.([0-9]+)`)
	mysqlVerMatch := mysqlVerRegexp.FindStringSubmatch(versionOutput)
	if mysqlVerMatch == nil {
		return 0, 0, 0, errors.New("No match")
	}
	vMaj, err = strconv.Atoi(mysqlVerMatch[2])
	if err != nil {
		return 0, 0, 0, err
	}
	vMin, err = strconv.Atoi(mysqlVerMatch[3])
	if err != nil {
		return 0, 0, 0, err
	}
	vPatch, err = strconv.Atoi(mysqlVerMatch[4])
	if err != nil {
		return 0, 0, 0, err
	}
	return
}
