package service

import (
	"fmt"
	"io"
	"net"

	"github.com/pingcap/tiup/components/playground/proc"
)

func init() {
	startAfter := []proc.ServiceID{
		proc.ServicePD,
		proc.ServicePDAPI,
		proc.ServiceTiKV,
		proc.ServiceTiKVWorker,
		proc.ServicePump,
	}
	for _, serviceID := range []proc.ServiceID{
		proc.ServiceTiDB,
		proc.ServiceTiDBSystem,
	} {
		serviceID := serviceID
		spec := Spec{
			ServiceID: serviceID,
			NewProc: func(rt Runtime, params NewProcParams) (proc.Process, error) {
				return newTiDBInstance(rt, serviceID, params)
			},
			PostScaleOut: postScaleOutTiDB,
		}
		if serviceID == proc.ServiceTiDBSystem {
			spec.StartAfter = startAfter
		}
		if serviceID == proc.ServiceTiDB {
			spec.StartAfter = append([]proc.ServiceID{proc.ServiceTiDBSystem}, startAfter...)
		}
		MustRegister(spec)
	}
}

func enableBinlog(rt Runtime) bool {
	if rt == nil {
		return false
	}
	cfg, ok := rt.BootConfig(proc.ServicePump)
	return ok && cfg.Num > 0
}

func newTiDBInstance(rt Runtime, serviceID proc.ServiceID, params NewProcParams) (proc.Process, error) {
	pds := ProcsOf[*proc.PDInstance](rt, proc.ServicePD, proc.ServicePDAPI)
	kvwrks := ProcsOf[*proc.TiKVWorkerInstance](rt, proc.ServiceTiKVWorker)
	shOpt := rt.SharedOptions()
	defaultPort := 4000
	if serviceID == proc.ServiceTiDBSystem {
		defaultPort = 3000
	}
	tdb := &proc.TiDBInstance{
		ShOpt:          shOpt,
		PDs:            pds,
		KVWorkers:      kvwrks,
		TiProxyCertDir: rt.DataDir(),
		EnableBinlog:   enableBinlog(rt),
		ProcessInfo: proc.ProcessInfo{
			UserBinPath:     params.Config.BinPath,
			ID:              params.ID,
			Dir:             params.Dir,
			Host:            params.Host,
			Port:            allocPort(params.Host, params.Config.Port, defaultPort, shOpt.PortOffset),
			StatusPort:      allocPort("0.0.0.0", 0, 10080, shOpt.PortOffset),
			ConfigPath:      params.Config.ConfigPath,
			RepoComponentID: proc.ComponentTiDB,
			Service:         serviceID,
		},
	}
	tdb.UpTimeout = params.Config.UpTimeout
	rt.AddProc(serviceID, tdb)
	return tdb, nil
}

func postScaleOutTiDB(w io.Writer, inst proc.Process) {
	if w == nil || inst == nil {
		return
	}
	tdb, ok := inst.(*proc.TiDBInstance)
	if !ok {
		return
	}

	host, port, err := net.SplitHostPort(tdb.Addr())
	if err != nil {
		fmt.Fprintf(w, "TiDB is up at %s\n", tdb.Addr())
		return
	}
	fmt.Fprintf(w, "Connect TiDB: %s --host %s --port %s -u root\n", "mysql --comments", host, port)
}
