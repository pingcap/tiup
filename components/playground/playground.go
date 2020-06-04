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
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
)

// Playground represent the playground of a cluster.
type Playground struct {
	bootOptions *bootOptions
	profile     *localdata.Profile
	port        int

	pds      []*instance.PDInstance
	tikvs    []*instance.TiKVInstance
	tidbs    []*instance.TiDBInstance
	tiflashs []*instance.TiFlashInstance

	idAlloc        map[string]int
	instanceWaiter errgroup.Group
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

func (p *Playground) handleScaleIn(w io.Writer, cmd *Command) error {
	fmt.Sprintln(w, "TODO")
	return nil
}

func (p *Playground) handleScaleOut(w io.Writer, cmd *Command) error {
	fmt.Sprintln(w, "TODO")
	return nil
}

func (p *Playground) handleCommand(cmd *Command, w io.Writer) error {
	fmt.Printf("receive command: %s\n", cmd.CommandType)
	switch cmd.CommandType {
	case DisplayCommandType:
		return p.handleDisplay(w)
	case ScaleInCommandType:
		return p.handleScaleIn(w, cmd)
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
		p.pds = append(p.pds, inst)
		for _, pd := range p.pds {
			pd.Join(p.pds)
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
	if options.monitor {
		if err := installIfMissing(p.profile, "prometheus", options.version); err != nil {
			return err
		}
		var pdAddrs, tidbAddrs, tikvAddrs, tiflashAddrs []string
		for _, pd := range p.pds {
			pdAddrs = append(pdAddrs, fmt.Sprintf("%s:%d", pd.Host, pd.StatusPort))
		}
		for _, db := range p.tidbs {
			tidbAddrs = append(tidbAddrs, fmt.Sprintf("%s:%d", db.Host, db.StatusPort))
		}
		for _, kv := range p.tikvs {
			tikvAddrs = append(tikvAddrs, fmt.Sprintf("%s:%d", kv.Host, kv.StatusPort))
		}
		for _, flash := range p.tiflashs {
			tiflashAddrs = append(tiflashAddrs, fmt.Sprintf("%s:%d", flash.Host, flash.StatusPort))
			tiflashAddrs = append(tiflashAddrs, fmt.Sprintf("%s:%d", flash.Host, flash.ProxyStatusPort))
		}

		dataDir := os.Getenv(localdata.EnvNameInstanceDataDir)
		promDir := filepath.Join(dataDir, "prometheus")
		port, cmd, err := startMonitor(ctx, options.host, promDir, tidbAddrs, tikvAddrs, pdAddrs, tiflashAddrs)
		if err != nil {
			return err
		}

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
			if err := checkStoreStatus(pdClient, kv.StoreAddr()); err != nil {
				lastErr = err
				break
			}
		}

		if lastErr != nil {
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
				if err := checkStoreStatus(pdClient, flash.StoreAddr()); err != nil {
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
				fmt.Print(color.GreenString("To view the monitor: http://%s:%d\n", monitorInfo.IP, monitorInfo.Port))
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
				_ = syscall.Kill(inst.Pid(), syscall.SIGKILL)
				return nil
			})
			if monitorCmd != nil {
				_ = syscall.Kill(monitorCmd.Process.Pid, sig)
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
