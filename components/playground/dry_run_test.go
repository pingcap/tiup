package main

import (
	"bytes"
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/stretchr/testify/require"
)

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
	require.Equal(t, `==> Download Packages:
  + tidb@v1.0.0

==> Existing Packages:
    pd@v1.0.0

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
