package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground-ng/proc"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestBuildBootPlan_DefaultNightly_NoLocalComponents(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "nightly",
		Host:    "127.0.0.1",
		Monitor: true,
	}
	applyServiceDefaultsForTest(t, opts)

	src := newTestComponentSource(t, map[string]string{"nightly": "v9.9.9"})
	plan := buildBootPlanForTest(t, opts, src)

	require.Equal(t, "nightly", plan.BootVersion)
	require.Equal(t, "127.0.0.1", plan.Host)
	require.Equal(t, proc.ModeNormal, plan.Shared.Mode)
	require.True(t, plan.Monitor)

	require.Equal(t, 1, plan.RequiredServices[proc.ServicePD.String()])
	require.Equal(t, 1, plan.RequiredServices[proc.ServiceTiKV.String()])
	require.Equal(t, 1, plan.RequiredServices[proc.ServiceTiDB.String()])

	require.Len(t, plan.Downloads, 6)
	require.Equal(t, proc.ComponentGrafana.String(), plan.Downloads[0].ComponentID)
	require.Equal(t, "v9.9.9", plan.Downloads[0].ResolvedVersion)
	require.Equal(t, proc.ComponentPD.String(), plan.Downloads[1].ComponentID)
	require.Equal(t, "v9.9.9", plan.Downloads[1].ResolvedVersion)
	require.Equal(t, proc.ComponentPrometheus.String(), plan.Downloads[2].ComponentID)
	require.Equal(t, "v9.9.9", plan.Downloads[2].ResolvedVersion)
	require.Equal(t, proc.ComponentTiDB.String(), plan.Downloads[3].ComponentID)
	require.Equal(t, "v9.9.9", plan.Downloads[3].ResolvedVersion)
	require.Equal(t, proc.ComponentTiFlash.String(), plan.Downloads[4].ComponentID)
	require.Equal(t, "v9.9.9", plan.Downloads[4].ResolvedVersion)
	require.Equal(t, proc.ComponentTiKV.String(), plan.Downloads[5].ComponentID)
	require.Equal(t, "v9.9.9", plan.Downloads[5].ResolvedVersion)

	require.Len(t, plan.Services, 7)

	// pd-0
	require.Equal(t, "pd-0", plan.Services[0].Name)
	require.Equal(t, proc.ServicePD.String(), plan.Services[0].ServiceID)
	require.Equal(t, proc.ComponentPD.String(), plan.Services[0].ComponentID)
	require.Equal(t, "nightly", plan.Services[0].DebugConstraint)
	require.Equal(t, "v9.9.9", plan.Services[0].ResolvedVersion)
	require.Equal(t, []string{}, plan.Services[0].StartAfterServices)
	require.Equal(t, "127.0.0.1", plan.Services[0].Shared.Host)
	require.Equal(t, 2380, plan.Services[0].Shared.Port)
	require.Equal(t, 2379, plan.Services[0].Shared.StatusPort)
	require.Equal(t, 2380, plan.Services[0].Shared.Ports[proc.PortNamePort])
	require.Equal(t, 2379, plan.Services[0].Shared.Ports[proc.PortNameStatusPort])
	require.NotNil(t, plan.Services[0].PD)
	require.Len(t, plan.Services[0].PD.InitialCluster, 1)
	require.Equal(t, "pd-0", plan.Services[0].PD.InitialCluster[0].Name)
	require.Equal(t, "127.0.0.1:2380", plan.Services[0].PD.InitialCluster[0].PeerAddr)
	require.True(t, plan.Services[0].PD.KVIsSingleReplica)

	// tikv-0
	require.Equal(t, "tikv-0", plan.Services[1].Name)
	require.Equal(t, proc.ServiceTiKV.String(), plan.Services[1].ServiceID)
	require.Equal(t, proc.ComponentTiKV.String(), plan.Services[1].ComponentID)
	require.Equal(t, "nightly", plan.Services[1].DebugConstraint)
	require.Equal(t, "v9.9.9", plan.Services[1].ResolvedVersion)
	require.Equal(t, []string{proc.ServicePD.String()}, plan.Services[1].StartAfterServices)
	require.Equal(t, 20160, plan.Services[1].Shared.Port)
	require.Equal(t, 20180, plan.Services[1].Shared.StatusPort)
	require.NotNil(t, plan.Services[1].TiKV)
	require.Equal(t, []string{"127.0.0.1:2379"}, plan.Services[1].TiKV.PDAddrs)
	require.Nil(t, plan.Services[1].TiKV.TSOAddrs)

	// tidb-0
	require.Equal(t, "tidb-0", plan.Services[2].Name)
	require.Equal(t, proc.ServiceTiDB.String(), plan.Services[2].ServiceID)
	require.Equal(t, proc.ComponentTiDB.String(), plan.Services[2].ComponentID)
	require.Equal(t, "nightly", plan.Services[2].DebugConstraint)
	require.Equal(t, "v9.9.9", plan.Services[2].ResolvedVersion)
	require.Equal(t, []string{proc.ServicePD.String(), proc.ServiceTiKV.String()}, plan.Services[2].StartAfterServices)
	require.Equal(t, 4000, plan.Services[2].Shared.Port)
	require.Equal(t, 10080, plan.Services[2].Shared.StatusPort)
	require.Equal(t, 4000, plan.Services[2].Shared.Ports[proc.PortNamePort])
	require.Equal(t, 10080, plan.Services[2].Shared.Ports[proc.PortNameStatusPort])
	require.NotNil(t, plan.Services[2].TiDB)
	require.Equal(t, []string{"127.0.0.1:2379"}, plan.Services[2].TiDB.PDAddrs)
	require.False(t, plan.Services[2].TiDB.EnableBinlog)
	require.Equal(t, "", plan.Services[2].TiDB.TiKVWorkerURL)

	// ng-monitoring-0
	require.Equal(t, "ng-monitoring-0", plan.Services[3].Name)
	require.Equal(t, proc.ServiceNGMonitoring.String(), plan.Services[3].ServiceID)
	require.Equal(t, proc.ComponentPrometheus.String(), plan.Services[3].ComponentID)
	require.Equal(t, "nightly", plan.Services[3].DebugConstraint)
	require.Equal(t, "v9.9.9", plan.Services[3].ResolvedVersion)
	require.Equal(t, []string{proc.ServicePD.String()}, plan.Services[3].StartAfterServices)
	require.Equal(t, 12020, plan.Services[3].Shared.Port)
	require.Equal(t, 12020, plan.Services[3].Shared.StatusPort)
	require.NotNil(t, plan.Services[3].NGMonitoring)
	require.Equal(t, []string{"127.0.0.1:2379"}, plan.Services[3].NGMonitoring.PDAddrs)

	// prometheus-0
	require.Equal(t, "prometheus-0", plan.Services[4].Name)
	require.Equal(t, proc.ServicePrometheus.String(), plan.Services[4].ServiceID)
	require.Equal(t, proc.ComponentPrometheus.String(), plan.Services[4].ComponentID)
	require.Equal(t, "nightly", plan.Services[4].DebugConstraint)
	require.Equal(t, "v9.9.9", plan.Services[4].ResolvedVersion)
	require.Equal(t, []string{}, plan.Services[4].StartAfterServices)
	require.Equal(t, 9090, plan.Services[4].Shared.Port)
	require.Equal(t, 9090, plan.Services[4].Shared.StatusPort)
	require.Equal(t, 9090, plan.Services[4].Shared.Ports[proc.PortNamePort])
	require.Equal(t, 9090, plan.Services[4].Shared.Ports[proc.PortNameStatusPort])

	// grafana-0
	require.Equal(t, "grafana-0", plan.Services[5].Name)
	require.Equal(t, proc.ServiceGrafana.String(), plan.Services[5].ServiceID)
	require.Equal(t, proc.ComponentGrafana.String(), plan.Services[5].ComponentID)
	require.Equal(t, "nightly", plan.Services[5].DebugConstraint)
	require.Equal(t, "v9.9.9", plan.Services[5].ResolvedVersion)
	require.Equal(t, []string{proc.ServicePrometheus.String()}, plan.Services[5].StartAfterServices)
	require.Equal(t, 3000, plan.Services[5].Shared.Port)
	require.NotNil(t, plan.Services[5].Grafana)
	require.Equal(t, "http://127.0.0.1:9090", plan.Services[5].Grafana.PrometheusURL)

	// tiflash-0
	require.Equal(t, "tiflash-0", plan.Services[6].Name)
	require.Equal(t, proc.ServiceTiFlash.String(), plan.Services[6].ServiceID)
	require.Equal(t, proc.ComponentTiFlash.String(), plan.Services[6].ComponentID)
	require.Equal(t, "nightly", plan.Services[6].DebugConstraint)
	require.Equal(t, "v9.9.9", plan.Services[6].ResolvedVersion)
	require.Equal(t, []string{proc.ServicePD.String(), proc.ServiceTiKV.String()}, plan.Services[6].StartAfterServices)
	require.Equal(t, 8123, plan.Services[6].Shared.Port)
	require.Equal(t, 8234, plan.Services[6].Shared.StatusPort)
	require.Equal(t, 8123, plan.Services[6].Shared.Ports[proc.PortNamePort])
	require.Equal(t, 8234, plan.Services[6].Shared.Ports[proc.PortNameStatusPort])
	require.Equal(t, 3930, plan.Services[6].Shared.Ports["service"])
	require.Equal(t, 9100, plan.Services[6].Shared.Ports["tcp"])
	require.Equal(t, 20170, plan.Services[6].Shared.Ports["proxy"])
	require.Equal(t, 20292, plan.Services[6].Shared.Ports["proxyStatus"])
	require.NotNil(t, plan.Services[6].TiFlash)
	require.Equal(t, []string{"127.0.0.1:2379"}, plan.Services[6].TiFlash.PDAddrs)
	require.Equal(t, 3930, plan.Services[6].TiFlash.ServicePort)
	require.Equal(t, 9100, plan.Services[6].TiFlash.TCPPort)
	require.Equal(t, 20170, plan.Services[6].TiFlash.ProxyPort)
	require.Equal(t, 20292, plan.Services[6].TiFlash.ProxyStatusPort)
}

func TestBuildBootPlan_DefaultNightly_PartialLocalComponents(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "nightly",
		Host:    "127.0.0.1",
		Monitor: true,
	}
	applyServiceDefaultsForTest(t, opts)

	src := newTestComponentSource(t, map[string]string{"nightly": "v9.9.9"})
	src.InstallComponent(proc.ComponentPD.String(), "v9.9.9")
	src.InstallComponent(proc.ComponentTiKV.String(), "v9.9.9")
	src.InstallComponent(proc.ComponentTiDB.String(), "v9.9.9")

	plan := buildBootPlanForTest(t, opts, src)

	require.Len(t, plan.Downloads, 3)
	require.Equal(t, proc.ComponentGrafana.String(), plan.Downloads[0].ComponentID)
	require.Equal(t, proc.ComponentPrometheus.String(), plan.Downloads[1].ComponentID)
	require.Equal(t, proc.ComponentTiFlash.String(), plan.Downloads[2].ComponentID)

	require.Len(t, plan.Services, 7)
	require.Equal(t, proc.ServicePD.String(), plan.Services[0].ServiceID)
	require.Equal(t, proc.ServiceTiKV.String(), plan.Services[1].ServiceID)
	require.Equal(t, proc.ServiceTiDB.String(), plan.Services[2].ServiceID)
}

func TestBuildBootPlan_DefaultVersion_NoLocalComponents(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "v8.0.0",
		Host:    "127.0.0.1",
		Monitor: true,
	}
	applyServiceDefaultsForTest(t, opts)

	src := newTestComponentSource(t, nil)
	plan := buildBootPlanForTest(t, opts, src)

	require.Equal(t, "v8.0.0", plan.BootVersion)

	require.Len(t, plan.Downloads, 6)
	require.Equal(t, proc.ComponentGrafana.String(), plan.Downloads[0].ComponentID)
	require.Equal(t, "v8.0.0", plan.Downloads[0].ResolvedVersion)
	require.Equal(t, proc.ComponentPD.String(), plan.Downloads[1].ComponentID)
	require.Equal(t, "v8.0.0", plan.Downloads[1].ResolvedVersion)
	require.Equal(t, proc.ComponentPrometheus.String(), plan.Downloads[2].ComponentID)
	require.Equal(t, "v8.0.0", plan.Downloads[2].ResolvedVersion)
	require.Equal(t, proc.ComponentTiDB.String(), plan.Downloads[3].ComponentID)
	require.Equal(t, "v8.0.0", plan.Downloads[3].ResolvedVersion)
	require.Equal(t, proc.ComponentTiFlash.String(), plan.Downloads[4].ComponentID)
	require.Equal(t, "v8.0.0", plan.Downloads[4].ResolvedVersion)
	require.Equal(t, proc.ComponentTiKV.String(), plan.Downloads[5].ComponentID)
	require.Equal(t, "v8.0.0", plan.Downloads[5].ResolvedVersion)
}

func TestBuildBootPlan_DefaultVersion_PartialLocalComponents(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "v8.0.0",
		Host:    "127.0.0.1",
		Monitor: true,
	}
	applyServiceDefaultsForTest(t, opts)

	src := newTestComponentSource(t, nil)
	src.InstallComponent(proc.ComponentTiDB.String(), "v8.0.0")
	src.InstallComponent(proc.ComponentTiKV.String(), "v8.0.0")
	src.InstallComponent(proc.ComponentTiFlash.String(), "v8.0.0")

	plan := buildBootPlanForTest(t, opts, src)

	require.Len(t, plan.Downloads, 3)
	require.Equal(t, proc.ComponentGrafana.String(), plan.Downloads[0].ComponentID)
	require.Equal(t, proc.ComponentPD.String(), plan.Downloads[1].ComponentID)
	require.Equal(t, proc.ComponentPrometheus.String(), plan.Downloads[2].ComponentID)
}

func TestBuildBootPlan_DefaultVersion_AllLocalComponents(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "v8.0.0",
		Host:    "127.0.0.1",
		Monitor: true,
	}
	applyServiceDefaultsForTest(t, opts)

	src := newTestComponentSource(t, nil)
	src.InstallComponent(proc.ComponentPD.String(), "v8.0.0")
	src.InstallComponent(proc.ComponentTiKV.String(), "v8.0.0")
	src.InstallComponent(proc.ComponentTiDB.String(), "v8.0.0")
	src.InstallComponent(proc.ComponentTiFlash.String(), "v8.0.0")
	src.InstallComponent(proc.ComponentPrometheus.String(), "v8.0.0", "ng-monitoring-server")
	src.InstallComponent(proc.ComponentGrafana.String(), "v8.0.0")

	plan := buildBootPlanForTest(t, opts, src)

	require.Empty(t, plan.Downloads)
}

func TestBuildBootPlan_DefaultVersion_UserBinPath_SkipsDownloadAndUsesBinPath(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "v8.0.0",
		Host:    "127.0.0.1",
		Monitor: true,
	}
	applyServiceDefaultsForTest(t, opts, "--db.binpath=/tmp/tidb-server")

	src := newTestComponentSource(t, nil)
	plan := buildBootPlanForTest(t, opts, src)

	// Still plans tidb service, but should not try to resolve/install the tidb component.
	require.Equal(t, proc.ServiceTiDB.String(), plan.Services[2].ServiceID)
	require.Equal(t, "/tmp/tidb-server", plan.Services[2].BinPath)

	require.Len(t, plan.Downloads, 5)
	require.Equal(t, proc.ComponentGrafana.String(), plan.Downloads[0].ComponentID)
	require.Equal(t, proc.ComponentPD.String(), plan.Downloads[1].ComponentID)
	require.Equal(t, proc.ComponentPrometheus.String(), plan.Downloads[2].ComponentID)
	require.Equal(t, proc.ComponentTiFlash.String(), plan.Downloads[3].ComponentID)
	require.Equal(t, proc.ComponentTiKV.String(), plan.Downloads[4].ComponentID)
}

func TestBuildBootPlan_DBCount3_MultiInstance(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "v8.0.0",
		Host:    "127.0.0.1",
		Monitor: true,
	}
	applyServiceDefaultsForTest(t, opts, "--db=3")

	src := newTestComponentSource(t, nil)
	plan := buildBootPlanForTest(t, opts, src)

	require.Len(t, plan.Services, 9)
	require.Equal(t, proc.ServicePD.String(), plan.Services[0].ServiceID)
	require.Equal(t, proc.ServiceTiKV.String(), plan.Services[1].ServiceID)

	require.Equal(t, "tidb-0", plan.Services[2].Name)
	require.Equal(t, proc.ServiceTiDB.String(), plan.Services[2].ServiceID)
	require.Equal(t, 4000, plan.Services[2].Shared.Port)
	require.Equal(t, 10080, plan.Services[2].Shared.StatusPort)

	require.Equal(t, "tidb-1", plan.Services[3].Name)
	require.Equal(t, proc.ServiceTiDB.String(), plan.Services[3].ServiceID)
	require.Equal(t, 4001, plan.Services[3].Shared.Port)
	require.Equal(t, 10081, plan.Services[3].Shared.StatusPort)

	require.Equal(t, "tidb-2", plan.Services[4].Name)
	require.Equal(t, proc.ServiceTiDB.String(), plan.Services[4].ServiceID)
	require.Equal(t, 4002, plan.Services[4].Shared.Port)
	require.Equal(t, 10082, plan.Services[4].Shared.StatusPort)

	require.Len(t, plan.Downloads, 6)
}

func TestBuildBootPlan_TiFlash0_DisablesTiFlashAndSkipsDownload(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "v8.0.0",
		Host:    "127.0.0.1",
		Monitor: true,
	}
	applyServiceDefaultsForTest(t, opts, "--tiflash=0")

	src := newTestComponentSource(t, nil)
	plan := buildBootPlanForTest(t, opts, src)

	require.Len(t, plan.Services, 6)
	require.Equal(t, proc.ServicePD.String(), plan.Services[0].ServiceID)
	require.Equal(t, proc.ServiceTiKV.String(), plan.Services[1].ServiceID)
	require.Equal(t, proc.ServiceTiDB.String(), plan.Services[2].ServiceID)
	require.Equal(t, proc.ServiceNGMonitoring.String(), plan.Services[3].ServiceID)
	require.Equal(t, proc.ServicePrometheus.String(), plan.Services[4].ServiceID)
	require.Equal(t, proc.ServiceGrafana.String(), plan.Services[5].ServiceID)

	require.Len(t, plan.Downloads, 5)
	require.Equal(t, proc.ComponentGrafana.String(), plan.Downloads[0].ComponentID)
	require.Equal(t, proc.ComponentPD.String(), plan.Downloads[1].ComponentID)
	require.Equal(t, proc.ComponentPrometheus.String(), plan.Downloads[2].ComponentID)
	require.Equal(t, proc.ComponentTiDB.String(), plan.Downloads[3].ComponentID)
	require.Equal(t, proc.ComponentTiKV.String(), plan.Downloads[4].ComponentID)
}

func TestBuildBootPlan_ModeTiKVSlim_Defaults(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeTiKVSlim,
			PDMode: "pd",
		},
		Version: "v8.0.0",
		Host:    "127.0.0.1",
		Monitor: true,
	}
	applyServiceDefaultsForTest(t, opts)

	src := newTestComponentSource(t, nil)
	plan := buildBootPlanForTest(t, opts, src)

	require.Equal(t, proc.ModeTiKVSlim, plan.Shared.Mode)

	require.Len(t, plan.Services, 5)
	require.Equal(t, proc.ServicePD.String(), plan.Services[0].ServiceID)
	require.Equal(t, proc.ServiceTiKV.String(), plan.Services[1].ServiceID)
	require.Equal(t, proc.ServiceNGMonitoring.String(), plan.Services[2].ServiceID)
	require.Equal(t, proc.ServicePrometheus.String(), plan.Services[3].ServiceID)
	require.Equal(t, proc.ServiceGrafana.String(), plan.Services[4].ServiceID)

	require.Len(t, plan.Downloads, 4)
	require.Equal(t, proc.ComponentGrafana.String(), plan.Downloads[0].ComponentID)
	require.Equal(t, proc.ComponentPD.String(), plan.Downloads[1].ComponentID)
	require.Equal(t, proc.ComponentPrometheus.String(), plan.Downloads[2].ComponentID)
	require.Equal(t, proc.ComponentTiKV.String(), plan.Downloads[3].ComponentID)
}

func TestBuildBootPlan_ModeCSE_Defaults(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeCSE,
			PDMode: "pd",
			CSE: proc.CSEOptions{
				S3Endpoint: "http://127.0.0.1:9000",
				Bucket:     "tiflash",
			},
		},
		Version: "v8.0.0",
		Host:    "127.0.0.1",
		Monitor: true,
	}
	applyServiceDefaultsForTest(t, opts)

	src := newTestComponentSource(t, nil)
	plan := buildBootPlanForTest(t, opts, src)

	require.Equal(t, proc.ModeCSE, plan.Shared.Mode)

	require.Len(t, plan.Services, 9)
	require.Equal(t, proc.ServicePD.String(), plan.Services[0].ServiceID)
	require.Equal(t, proc.ServiceTiKV.String(), plan.Services[1].ServiceID)
	require.Equal(t, proc.ServiceTiKVWorker.String(), plan.Services[2].ServiceID)
	require.Equal(t, proc.ServiceTiDB.String(), plan.Services[3].ServiceID)
	require.Equal(t, proc.ServiceNGMonitoring.String(), plan.Services[4].ServiceID)
	require.Equal(t, proc.ServicePrometheus.String(), plan.Services[5].ServiceID)
	require.Equal(t, proc.ServiceGrafana.String(), plan.Services[6].ServiceID)
	require.Equal(t, proc.ServiceTiFlashCompute.String(), plan.Services[7].ServiceID)
	require.Equal(t, proc.ServiceTiFlashWrite.String(), plan.Services[8].ServiceID)

	require.Len(t, plan.Downloads, 6)
	require.Equal(t, proc.ComponentGrafana.String(), plan.Downloads[0].ComponentID)
	require.Equal(t, proc.ComponentPD.String(), plan.Downloads[1].ComponentID)
	require.Equal(t, proc.ComponentPrometheus.String(), plan.Downloads[2].ComponentID)
	require.Equal(t, proc.ComponentTiDB.String(), plan.Downloads[3].ComponentID)
	require.Equal(t, proc.ComponentTiFlash.String(), plan.Downloads[4].ComponentID)
	require.Equal(t, proc.ComponentTiKV.String(), plan.Downloads[5].ComponentID)
}

func TestBuildBootPlan_ModeCSE_TiFlashConfigPropagatesToWriteCompute(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeCSE,
			PDMode: "pd",
			CSE: proc.CSEOptions{
				S3Endpoint: "http://127.0.0.1:9000",
				Bucket:     "tiflash",
			},
		},
		Version: "v8.0.0",
		Host:    "127.0.0.1",
		Monitor: false,
	}
	applyServiceDefaultsForTest(t, opts, "--tiflash.config=/tmp/tiflash.toml")

	plan := buildBootPlanForTest(t, opts, nil)

	require.Equal(t, proc.ServiceTiFlashCompute.String(), plan.Services[4].ServiceID)
	require.Equal(t, "/tmp/tiflash.toml", plan.Services[4].Shared.ConfigPath)
	require.Equal(t, proc.ServiceTiFlashWrite.String(), plan.Services[5].ServiceID)
	require.Equal(t, "/tmp/tiflash.toml", plan.Services[5].Shared.ConfigPath)
}

func TestBuildBootPlan_MonitorDisabled_ExcludesMonitoringServices(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "v8.0.0",
		Host:    "127.0.0.1",
		Monitor: false,
	}
	applyServiceDefaultsForTest(t, opts)

	plan := buildBootPlanForTest(t, opts, nil)

	require.False(t, plan.Monitor)
	require.Len(t, plan.Services, 4)
	require.Equal(t, proc.ServicePD.String(), plan.Services[0].ServiceID)
	require.Equal(t, proc.ServiceTiKV.String(), plan.Services[1].ServiceID)
	require.Equal(t, proc.ServiceTiDB.String(), plan.Services[2].ServiceID)
	require.Equal(t, proc.ServiceTiFlash.String(), plan.Services[3].ServiceID)
}

func TestTiFlashDefaultNum_DisabledOnOldVersion(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "v6.5.0",
		Host:    "127.0.0.1",
		Monitor: false,
	}
	applyServiceDefaultsForTest(t, opts)

	require.Equal(t, 0, opts.Service(proc.ServiceTiFlash).Num)

	plan := buildBootPlanForTest(t, opts, nil)
	require.Equal(t, 0, plan.DebugServiceConfigs[proc.ServiceTiFlash.String()].Num)
}

func TestBuildBootPlan_TiFlashDisaggRequiresTiDB(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeCSE,
			PDMode: "pd",
			CSE: proc.CSEOptions{
				S3Endpoint: "http://127.0.0.1:9000",
				Bucket:     "tiflash",
			},
		},
		Version: "nightly",
		Host:    "127.0.0.1",
		Monitor: false,
	}
	applyServiceDefaultsForTest(t, opts)

	// Simulate "--db 0".
	opts.Service(proc.ServiceTiDB).Num = 0

	plan := buildBootPlanForTest(t, opts, nil)

	_, ok := plan.DebugServiceConfigs[proc.ServiceTiFlashWrite.String()]
	require.False(t, ok)
	_, ok = plan.DebugServiceConfigs[proc.ServiceTiFlashCompute.String()]
	require.False(t, ok)
}

func TestPDAPIDefaults_InheritFromPDInMicroservicesMode(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "ms",
		},
		Version: "nightly",
		Host:    "127.0.0.1",
		Monitor: false,
	}
	applyServiceDefaultsForTest(t, opts,
		"--pd=2",
		"--pd.host=1.2.3.4",
		"--pd.port=1234",
		"--pd.config=/tmp/pd.toml",
		"--pd.binpath=/tmp/pd",
	)

	pdAPI := opts.Service(proc.ServicePDAPI)
	require.NotNil(t, pdAPI)
	require.Equal(t, 2, pdAPI.Num)
	require.Equal(t, "1.2.3.4", pdAPI.Host)
	require.Equal(t, 1234, pdAPI.Port)
	require.Equal(t, "/tmp/pd.toml", pdAPI.ConfigPath)
	require.Equal(t, "/tmp/pd", pdAPI.BinPath)

	plan := buildBootPlanForTest(t, opts, nil)

	_, ok := plan.DebugServiceConfigs[proc.ServicePD.String()]
	require.False(t, ok)
	cfg, ok := plan.DebugServiceConfigs[proc.ServicePDAPI.String()]
	require.True(t, ok)
	require.Equal(t, 2, cfg.Num)
	require.Equal(t, 1, plan.RequiredServices[proc.ServicePDAPI.String()])
}

func TestBuildBootPlan_PDModeMS_PlansMicroservicePortsWithStatusAlias(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "ms",
		},
		Version: "nightly",
		Host:    "127.0.0.1",
		Monitor: false,
	}
	applyServiceDefaultsForTest(t, opts)

	plan := buildBootPlanForTest(t, opts, nil)

	var tso *ServicePlan
	for i := range plan.Services {
		if plan.Services[i].ServiceID == proc.ServicePDTSO.String() {
			tso = &plan.Services[i]
			break
		}
	}
	require.NotNil(t, tso)
	require.Equal(t, tso.Shared.StatusPort, tso.Shared.Port)
	require.Equal(t, tso.Shared.StatusPort, tso.Shared.Ports[proc.PortNameStatusPort])
	require.Equal(t, tso.Shared.Port, tso.Shared.Ports[proc.PortNamePort])
}

func TestPDAPIDefaults_OverridePD(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "ms",
		},
		Version: "nightly",
		Host:    "127.0.0.1",
		Monitor: false,
	}
	applyServiceDefaultsForTest(t, opts,
		"--pd.host=1.2.3.4",
		"--pd.port=1234",
		"--pd.api.host=5.6.7.8",
		"--pd.api.port=2345",
	)

	pdAPI := opts.Service(proc.ServicePDAPI)
	require.NotNil(t, pdAPI)
	require.Equal(t, "5.6.7.8", pdAPI.Host)
	require.Equal(t, 2345, pdAPI.Port)
}

func TestBuildBootPlan_ModeDisAgg_Defaults(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeDisAgg,
			PDMode: "pd",
			CSE: proc.CSEOptions{
				S3Endpoint: "http://127.0.0.1:9000",
				Bucket:     "tiflash",
			},
		},
		Version: "nightly",
		Host:    "127.0.0.1",
		Monitor: false,
	}
	applyServiceDefaultsForTest(t, opts)

	plan := buildBootPlanForTest(t, opts, nil)
	require.Equal(t, proc.ModeDisAgg, plan.Shared.Mode)

	require.Len(t, plan.Services, 5)
	require.Equal(t, proc.ServicePD.String(), plan.Services[0].ServiceID)
	require.Equal(t, proc.ServiceTiKV.String(), plan.Services[1].ServiceID)
	require.Equal(t, proc.ServiceTiDB.String(), plan.Services[2].ServiceID)
	require.Equal(t, proc.ServiceTiFlashCompute.String(), plan.Services[3].ServiceID)
	require.Equal(t, proc.ServiceTiFlashWrite.String(), plan.Services[4].ServiceID)
}

func TestBuildBootPlan_ModeNextGen_Defaults(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNextGen,
			PDMode: "pd",
			CSE: proc.CSEOptions{
				S3Endpoint: "http://127.0.0.1:9000",
				Bucket:     "tiflash",
			},
		},
		Version: "nightly",
		Host:    "127.0.0.1",
		Monitor: false,
	}
	applyServiceDefaultsForTest(t, opts)

	plan := buildBootPlanForTest(t, opts, nil)
	require.Equal(t, proc.ModeNextGen, plan.Shared.Mode)

	require.Len(t, plan.Services, 5)
	require.Equal(t, proc.ServicePD.String(), plan.Services[0].ServiceID)
	require.Equal(t, proc.ServiceTiKV.String(), plan.Services[1].ServiceID)

	require.Equal(t, proc.ServiceTiKVWorker.String(), plan.Services[2].ServiceID)
	require.Equal(t, proc.ComponentTiKVWorker.String(), plan.Services[2].ComponentID)

	require.Equal(t, proc.ServiceTiDBSystem.String(), plan.Services[3].ServiceID)
	require.Equal(t, proc.ServiceTiDB.String(), plan.Services[4].ServiceID)
}

func TestPortPlanner_PortConflictNone_WildcardConflictsWithSpecificHost(t *testing.T) {
	p := newPortPlanner(PortConflictNone)

	a, err := p.alloc("127.0.0.1", 10080, 0)
	require.NoError(t, err)

	b, err := p.alloc("0.0.0.0", 10080, 0)
	require.NoError(t, err)
	require.NotEqual(t, a, b)

	c, err := p.alloc("127.0.0.1", 10080, 0)
	require.NoError(t, err)
	require.NotEqual(t, a, c)
	require.NotEqual(t, b, c)
}

func TestBuildBootPlan_PortConflictNone_UniquePDPorts(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "nightly",
		Host:    "127.0.0.1",
		Monitor: false,
	}
	applyServiceDefaultsForTest(t, opts, "--pd=2")

	plan := buildBootPlanForTest(t, opts, nil)

	require.Len(t, plan.Services, 5)
	require.Equal(t, "pd-0", plan.Services[0].Name)
	require.Equal(t, proc.ServicePD.String(), plan.Services[0].ServiceID)
	require.Equal(t, 2380, plan.Services[0].Shared.Port)
	require.Equal(t, 2379, plan.Services[0].Shared.StatusPort)

	require.Equal(t, "pd-1", plan.Services[1].Name)
	require.Equal(t, proc.ServicePD.String(), plan.Services[1].ServiceID)
	require.Equal(t, 2381, plan.Services[1].Shared.Port)
	require.Equal(t, 2382, plan.Services[1].Shared.StatusPort)
}

func TestWriteDryRun_Text(t *testing.T) {
	plan := BootPlan{
		DataDir:     "/data",
		BootVersion: "nightly",
		Host:        "127.0.0.1",
		Shared: proc.SharedOptions{
			Mode:               proc.ModeCSE,
			PDMode:             "pd",
			PortOffset:         123,
			HighPerf:           true,
			EnableTiKVColumnar: true,
			ForcePull:          true,
			CSE: proc.CSEOptions{
				S3Endpoint: "https://s3.example.com",
				Bucket:     "my-bucket",
				AccessKey:  "fake-access-key",
				SecretKey:  "fake-secret-key",
			},
		},
		Monitor:     true,
		GrafanaPort: 3000,
		Downloads: []DownloadPlan{
			{ComponentID: "tidb", ResolvedVersion: "v1.0.0", DebugReason: "missing_binary", DebugBinPath: "/home/tidb-server"},
		},
		Services: []ServicePlan{
			{
				Name:               "pd-0",
				ServiceID:          proc.ServicePD.String(),
				ComponentID:        proc.ComponentPD.String(),
				ResolvedVersion:    "v1.0.0",
				StartAfterServices: []string{proc.ServiceTiKV.String()},
				Shared:             ServiceSharedPlan{Dir: "/data/pd-0", Host: "127.0.0.1", Port: 2380, StatusPort: 2379},
			},
			{
				Name:            "tidb-0",
				ServiceID:       proc.ServiceTiDB.String(),
				ComponentID:     proc.ComponentTiDB.String(),
				ResolvedVersion: "v1.0.0",
				BinPath:         "/usr/local/bin/tidb-server",
				Shared:          ServiceSharedPlan{Dir: "/data/tidb-0", Host: "127.0.0.1", Port: 4000, StatusPort: 10080},
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, writeDryRun(&buf, plan, "text"))
	require.Equal(t, `==> Existing Packages:
    pd@v1.0.0

==> Download Packages:
  + tidb@v1.0.0

==> Start Services:
  + pd-0@v1.0.0
    127.0.0.1:2380,2379
    Start after: tikv
  + tidb-0@v1.0.0 (use /usr/local/bin/tidb-server)
    127.0.0.1:4000,10080
`, buf.String())
}

func TestWriteDryRun_Text_ShowsComponentHintWhenDifferent(t *testing.T) {
	plan := BootPlan{
		Services: []ServicePlan{
			{
				Name:            "ng-monitoring-0",
				ServiceID:       proc.ServiceNGMonitoring.String(),
				ComponentID:     proc.ComponentPrometheus.String(),
				ResolvedVersion: "v1.0.0",
				Shared:          ServiceSharedPlan{Host: "127.0.0.1", Port: 12020, StatusPort: 12020},
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, writeDryRun(&buf, plan, "text"))
	require.Equal(t, `==> Existing Packages:
    prometheus@v1.0.0

==> Start Services:
  + prometheus/ng-monitoring-0@v1.0.0
    127.0.0.1:12020
`, buf.String())
}

func TestWriteDryRun_JSON(t *testing.T) {
	plan := BootPlan{
		DataDir:     "/data",
		BootVersion: "nightly",
		Host:        "127.0.0.1",
		Shared:      proc.SharedOptions{Mode: proc.ModeNormal, PDMode: "pd"},
	}

	var buf bytes.Buffer
	require.NoError(t, writeDryRun(&buf, plan, "json"))
	require.Equal(t, `{
  "DataDir": "/data",
  "BootVersion": "nightly",
  "Host": "127.0.0.1",
  "Shared": {
    "HighPerf": false,
    "CSE": {
      "S3Endpoint": "",
      "Bucket": "",
      "AccessKey": "",
      "SecretKey": ""
    },
    "PDMode": "pd",
    "Mode": "tidb",
    "PortOffset": 0,
    "EnableTiKVColumnar": false,
    "ForcePull": false
  },
  "Monitor": false,
  "GrafanaPort": 0,
  "Downloads": null,
  "Services": null,
  "RequiredServices": null,
  "DebugServiceConfigs": null
}
`, buf.String())
}

func TestWriteDryRun_JSON_RedactsSecrets(t *testing.T) {
	plan := BootPlan{
		Shared: proc.SharedOptions{
			Mode: proc.ModeCSE,
			CSE: proc.CSEOptions{
				AccessKey: "access-KEY-123",
				SecretKey: "secret-KEY-456",
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, writeDryRun(&buf, plan, "json"))
	require.Equal(t, `{
  "DataDir": "",
  "BootVersion": "",
  "Host": "",
  "Shared": {
    "HighPerf": false,
    "CSE": {
      "S3Endpoint": "",
      "Bucket": "",
      "AccessKey": "***",
      "SecretKey": "***"
    },
    "PDMode": "",
    "Mode": "tidb-cse",
    "PortOffset": 0,
    "EnableTiKVColumnar": false,
    "ForcePull": false
  },
  "Monitor": false,
  "GrafanaPort": 0,
  "Downloads": null,
  "Services": null,
  "RequiredServices": null,
  "DebugServiceConfigs": null
}
`, buf.String())
}

func TestWriteDryRun_JSON_OmitsNilOneOfFields(t *testing.T) {
	plan := BootPlan{
		Services: []ServicePlan{
			{
				Name:            "pd-0",
				ServiceID:       proc.ServicePD.String(),
				ComponentID:     proc.ComponentPD.String(),
				ResolvedVersion: "v1.0.0",
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, writeDryRun(&buf, plan, "json"))
	require.Equal(t, `{
  "DataDir": "",
  "BootVersion": "",
  "Host": "",
  "Shared": {
    "HighPerf": false,
    "CSE": {
      "S3Endpoint": "",
      "Bucket": "",
      "AccessKey": "",
      "SecretKey": ""
    },
    "PDMode": "",
    "Mode": "",
    "PortOffset": 0,
    "EnableTiKVColumnar": false,
    "ForcePull": false
  },
  "Monitor": false,
  "GrafanaPort": 0,
  "Downloads": null,
  "Services": [
    {
      "Name": "pd-0",
      "ServiceID": "pd",
      "StartAfterServices": null,
      "ComponentID": "pd",
      "ResolvedVersion": "v1.0.0",
      "BinPath": "",
      "Shared": {
        "Dir": "",
        "Host": "",
        "Port": 0,
        "StatusPort": 0,
        "ConfigPath": "",
        "UpTimeout": 0
      },
      "DebugConstraint": ""
    }
  ],
  "RequiredServices": null,
  "DebugServiceConfigs": null
}
`, buf.String())
}

func TestWriteDryRun_JSON_MapOrderIsStable(t *testing.T) {
	plan := BootPlan{
		RequiredServices: map[string]int{
			"zzz": 1,
			"aaa": 2,
		},
		DebugServiceConfigs: map[string]proc.Config{
			"zzz_cfg": {Num: 1},
			"aaa_cfg": {Num: 2},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, writeDryRun(&buf, plan, "json"))
	require.Equal(t, `{
  "DataDir": "",
  "BootVersion": "",
  "Host": "",
  "Shared": {
    "HighPerf": false,
    "CSE": {
      "S3Endpoint": "",
      "Bucket": "",
      "AccessKey": "",
      "SecretKey": ""
    },
    "PDMode": "",
    "Mode": "",
    "PortOffset": 0,
    "EnableTiKVColumnar": false,
    "ForcePull": false
  },
  "Monitor": false,
  "GrafanaPort": 0,
  "Downloads": null,
  "Services": null,
  "RequiredServices": {
    "aaa": 2,
    "zzz": 1
  },
  "DebugServiceConfigs": {
    "aaa_cfg": {
      "ConfigPath": "",
      "BinPath": "",
      "Num": 2,
      "Host": "",
      "Port": 0,
      "UpTimeout": 0,
      "Version": ""
    },
    "zzz_cfg": {
      "ConfigPath": "",
      "BinPath": "",
      "Num": 1,
      "Host": "",
      "Port": 0,
      "UpTimeout": 0,
      "Version": ""
    }
  }
}
`, buf.String())
}

func TestWriteDryRun_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	require.Error(t, writeDryRun(&buf, BootPlan{}, "xml"))
}

func TestWriteDryRun_NilWriter(t *testing.T) {
	require.Error(t, writeDryRun(nil, BootPlan{}, "text"))
}

func TestResolveVersionConstraint_UsesLatestAliasByDefault(t *testing.T) {
	options := &BootOptions{}
	got, err := resolveVersionConstraint(proc.ServiceTiProxy, options)
	require.NoError(t, err)
	require.Equal(t, utils.LatestVersionAlias, got)
}

func TestResolveVersionConstraint_ServiceOverrideAndBind(t *testing.T) {
	options := &BootOptions{Version: "latest-" + utils.NextgenVersionAlias}
	options.Service(proc.ServiceTiProxy).Version = "v9.9.9"
	options.Service(proc.ServicePrometheus).Version = "v0.0.0-should-be-ignored"

	got, err := resolveVersionConstraint(proc.ServiceTiProxy, options)
	require.NoError(t, err)
	require.Equal(t, "v9.9.9", got)

	got, err = resolveVersionConstraint(proc.ServicePrometheus, options)
	require.NoError(t, err)
	require.Equal(t, utils.LatestVersionAlias, got)
}

func newTestFlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func applyServiceDefaultsForTest(t *testing.T, opts *BootOptions, args ...string) {
	t.Helper()

	fs := newTestFlagSet()
	registerServiceFlags(fs, opts)
	require.NoError(t, fs.Parse(args))
	require.NoError(t, applyServiceDefaults(fs, opts))
}

func buildBootPlanForTest(t *testing.T, opts *BootOptions, src ComponentSource) BootPlan {
	t.Helper()

	plan, err := BuildBootPlan(opts, bootPlannerConfig{
		dataDir:            t.TempDir(),
		portConflictPolicy: PortConflictNone,
		advertiseHost:      func(listen string) string { return listen },
		componentSource:    src,
	})
	require.NoError(t, err)
	return plan
}

type testComponentSource struct {
	t *testing.T

	root string

	installed map[string]bool // key: component@resolved

	// resolvedByConstraint maps "constraint" -> "resolved". When absent, the
	// constraint itself is treated as resolved.
	resolvedByConstraint map[string]string
}

func newTestComponentSource(t *testing.T, resolvedByConstraint map[string]string) *testComponentSource {
	t.Helper()
	if resolvedByConstraint == nil {
		resolvedByConstraint = make(map[string]string)
	}
	return &testComponentSource{
		t:                    t,
		root:                 t.TempDir(),
		installed:            make(map[string]bool),
		resolvedByConstraint: resolvedByConstraint,
	}
}

func (s *testComponentSource) ResolveVersion(component, constraint string) (string, error) {
	_ = component

	c := strings.TrimSpace(constraint)
	if c == "" {
		return "", nil
	}
	if resolved := s.resolvedByConstraint[c]; resolved != "" {
		return resolved, nil
	}
	return c, nil
}

func (s *testComponentSource) PlanInstall(serviceID proc.ServiceID, component, resolved string, forcePull bool) (*DownloadPlan, error) {
	if component == "" {
		return nil, errors.New("component is empty")
	}
	if resolved == "" {
		return nil, errors.Errorf("component %s resolved version is empty", component)
	}

	baseBinPath, err := s.BinaryPath(component, resolved)
	return planInstallByResolvedBinaryPath(serviceID, component, resolved, baseBinPath, err, forcePull), nil
}

func (s *testComponentSource) EnsureInstalled(component, resolved string) error {
	_ = component
	_ = resolved
	return nil
}

func (s *testComponentSource) BinaryPath(component, resolved string) (string, error) {
	if component == "" {
		return "", errors.New("component is empty")
	}
	if resolved == "" {
		return "", errors.Errorf("component %s resolved version is empty", component)
	}

	if !s.installed[component+"@"+resolved] {
		return "", errors.New("not installed")
	}

	return filepath.Join(s.root, component, resolved, baseBinaryNameForComponent(component)), nil
}

func (s *testComponentSource) InstallComponent(component, resolved string, extraBinaries ...string) {
	if s == nil || s.t == nil {
		return
	}

	s.t.Helper()

	s.installed[component+"@"+resolved] = true

	dir := filepath.Join(s.root, component, resolved)
	require.NoError(s.t, os.MkdirAll(dir, 0o755))

	base := filepath.Join(dir, baseBinaryNameForComponent(component))
	require.NoError(s.t, os.WriteFile(base, []byte("bin"), 0o755))

	for _, name := range extraBinaries {
		require.NoError(s.t, os.WriteFile(filepath.Join(dir, name), []byte("bin"), 0o755))
	}
}

func baseBinaryNameForComponent(component string) string {
	switch strings.TrimSpace(component) {
	case proc.ComponentPD.String():
		return "pd-server"
	case proc.ComponentTiKV.String():
		return "tikv-server"
	case proc.ComponentTiDB.String():
		return "tidb-server"
	case proc.ComponentTiFlash.String():
		return "tiflash"
	case proc.ComponentPrometheus.String():
		return "prometheus"
	case proc.ComponentGrafana.String():
		return "grafana-server"
	case proc.ComponentTiKVWorker.String():
		return "tikv-worker"
	default:
		// Best effort fallback for tests that don't care about the exact binary name.
		return component
	}
}
