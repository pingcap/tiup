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
	"strconv"
	"strings"
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
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
)

// Playground represent the playground of a cluster.
type Playground struct {
	booted      bool
	bootOptions *bootOptions
	profile     *localdata.Profile
	port        int

	pds      []*instance.PDInstance
	tikvs    []*instance.TiKVInstance
	tidbs    []*instance.TiDBInstance
	tiflashs []*instance.TiFlashInstance

	idAlloc        map[string]int
	instanceWaiter errgroup.Group

	monitor *monitor
}

// NewPlayground create a Playground instance.
func NewPlayground(port int) *Playground {
	return &Playground{
		port:    port,
		profile: localdata.InitProfile(),
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
	header := []interface{}{"Pid", "Role"}
	t.AddHeader(header...)

	err = p.WalkInstances(func(componentID string, ins instance.Instance) error {
		row := make([]interface{}, len(header))
		row[0] = strconv.Itoa(ins.Pid())
		row[1] = componentID
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

func (p *Playground) sanitizeConfig(boot instance.Config, cfg *instance.Config) {
	if cfg.BinPath == "" {
		cfg.BinPath = boot.BinPath
	}
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = boot.ConfigPath
	}
	if cfg.Host == "" {
		cfg.Host = boot.ConfigPath
	}
}

func (p *Playground) sanitizeComponentConfig(cid string, cfg *instance.Config) {
	switch cid {
	case "pd":
		p.sanitizeConfig(p.bootOptions.pd, cfg)
	case "tikv":
		p.sanitizeConfig(p.bootOptions.tikv, cfg)
	case "tidb":
		p.sanitizeConfig(p.bootOptions.tidb, cfg)
	case "tiflash":
		p.sanitizeConfig(p.bootOptions.tiflash, cfg)
	default:
		fmt.Printf("unknow %s in sanitizeConfig", cid)
	}
}

func (p *Playground) handleScaleOut(w io.Writer, cmd *Command) error {
	// Ignore Config.Num, alway one command as scale out one instance.
	p.sanitizeComponentConfig(cmd.ComponentID, &cmd.Config)
	inst, err := p.addInstance(cmd.ComponentID, cmd.Config)
	if err != nil {
		return errors.AddStack(err)
	}

	err = inst.Start(context.Background(), v0manifest.Version(p.bootOptions.version))
	if err != nil {
		return errors.AddStack(err)
	}

	p.instanceWaiter.Go(func() error {
		return inst.Wait()
	})

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
	for _, ins := range p.tidbs {
		err := fn("tidb", ins)
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

func (p *Playground) addInstance(componentID string, cfg instance.Config) (ins instance.Instance, err error) {
	if cfg.BinPath != "" {
		cfg.BinPath = getAbsolutePath(cfg.BinPath)
	}

	if cfg.ConfigPath != "" {
		cfg.ConfigPath = getAbsolutePath(cfg.ConfigPath)
	}

	err = installIfMissing(p.profile, componentID, p.bootOptions.version)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to install %s", componentID)
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
		inst := instance.NewTiDBInstance(cfg.BinPath, dir, host, cfg.ConfigPath, id, p.pds)
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
	default:
		return nil, errors.Errorf("unknow component: %s", componentID)
	}

	return
}

func (p *Playground) bootCluster(options *bootOptions) error {
	p.bootOptions = options

	if options.pd.Num < 1 || options.tidb.Num < 1 || options.tikv.Num < 1 {
		return fmt.Errorf("all components count must be great than 0 (tidb=%v, tikv=%v, pd=%v)",
			options.pd.Num, options.tikv.Num, options.pd.Num)
	}

	if options.version != "" && semver.Compare("v3.1.0", options.version) > 0 && options.tiflash.Num != 0 {
		fmt.Println(color.YellowString("Warning: current version %s doesn't support TiFlash", options.version))
		options.tiflash.Num = 0
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
	for i := 0; i < options.tidb.Num; i++ {
		_, err := p.addInstance("tidb", options.tidb)
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

	monitorInfo := struct {
		IP         string `json:"ip"`
		Port       int    `json:"port"`
		BinaryPath string `json:"binary_path"`
	}{}

	var monitorCmd *exec.Cmd
	var grafana *grafana
	if options.monitor {
		// set up prometheus
		if err := installIfMissing(p.profile, "prometheus", options.version); err != nil {
			return err
		}

		dataDir := os.Getenv(localdata.EnvNameInstanceDataDir)
		promDir := filepath.Join(dataDir, "prometheus")

		monitor := newMonitor()
		port, cmd, err := monitor.startMonitor(ctx, options.version, options.host, promDir)
		if err != nil {
			return err
		}
		p.monitor = monitor

		monitorInfo.IP = options.host
		monitorInfo.BinaryPath = promDir
		monitorInfo.Port = port

		monitorCmd = cmd
		go func() {
			log, err := os.OpenFile(filepath.Join(promDir, "prom.log"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, os.ModePerm)
			if err != nil {
				fmt.Println("Monitor system start failed", err)
				return
			}
			defer log.Close()

			cmd.Stderr = log
			cmd.Stdout = os.Stdout

			if err := cmd.Start(); err != nil {
				fmt.Println("Monitor system start failed", err)
				return
			}
		}()

		// set up grafana
		if err := installIfMissing(p.profile, "grafana", options.version); err != nil {
			return err
		}
		installPath, err := p.profile.ComponentInstalledPath("grafana", v0manifest.Version(options.version))
		if err != nil {
			return errors.AddStack(err)
		}

		dataDir = os.Getenv(localdata.EnvNameInstanceDataDir)
		grafanaDir := filepath.Join(dataDir, "grafana")

		cmd = exec.Command("cp", "-r", installPath, grafanaDir)
		err = cmd.Run()
		if err != nil {
			return errors.AddStack(err)
		}

		dashboardDir := filepath.Join(grafanaDir, "dashboards")
		err = os.MkdirAll(dashboardDir, 0755)
		if err != nil {
			return errors.AddStack(err)
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
			return errors.AddStack(err)
		}

		err = replaceDatasource(dashboardDir, clusterName)
		if err != nil {
			return errors.AddStack(err)
		}

		grafana = newGrafana(options.version, options.host)
		err = grafana.start(ctx, grafanaDir, fmt.Sprintf("http://%s:%d", monitorInfo.IP, monitorInfo.Port))
		if err != nil {
			return errors.AddStack(err)
		}
	}

	// Start all instance except tiflash.
	err := p.WalkInstances(func(cid string, ins instance.Instance) error {
		if cid == "tiflash" {
			return nil
		}

		return ins.Start(ctx, v0manifest.Version(options.version))
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
				if err := flash.Start(ctx, v0manifest.Version(options.version)); err != nil {
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

	if monitorInfo.IP != "" && len(p.pds) != 0 {
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
		_ = p.WalkInstances(func(_ string, inst instance.Instance) error {
			_ = syscall.Kill(inst.Pid(), syscall.SIGKILL)
			return nil
		})

		if monitorCmd != nil {
			_ = syscall.Kill(monitorCmd.Process.Pid, syscall.SIGKILL)
		}

		fmt.Println("Wait all processes terminated")
		_ = p.WalkInstances(func(_ string, inst instance.Instance) error {
			_ = inst.Wait()
			return nil
		})
		if monitorCmd != nil {
			_ = monitorCmd.Wait()
		}
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
		if sig != syscall.SIGINT {
			_ = p.WalkInstances(func(_ string, inst instance.Instance) error {
				_ = syscall.Kill(inst.Pid(), sig)
				return nil
			})
			if monitorCmd != nil {
				_ = syscall.Kill(monitorCmd.Process.Pid, sig)
			}
			if grafana != nil {
				_ = syscall.Kill(grafana.cmd.Process.Pid, sig)
			}
		}
	}()

	go func() {
		// fmt.Printf("serve at :%d\n", p.port)
		err := p.listenAndServeHTTP()
		if err != nil {
			fmt.Printf("listenAndServeHTTP quit: %s\n", err)
		}
	}()

	_ = p.WalkInstances(func(_ string, inst instance.Instance) error {
		p.instanceWaiter.Go(func() error {
			return inst.Wait()
		})
		return nil
	})

	logIfErr(p.renderSDFile())

	if grafana != nil {
		p.instanceWaiter.Go(func() error {
			err := grafana.cmd.Wait()
			return err
		})
		fmt.Print(color.GreenString("To view the Grafana: http://%s:%d\n", grafana.host, grafana.port))
	}

	err = p.instanceWaiter.Wait()
	if err != nil {
		return err
	}

	if monitorCmd != nil {
		if err := monitorCmd.Wait(); err != nil {
			fmt.Println("Monitor system wait failed", err)
		}
	}

	return nil
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

func logIfErr(err error) {
	if err != nil {
		fmt.Println(err)
	}
}
