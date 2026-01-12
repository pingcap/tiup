package main

import (
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
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
