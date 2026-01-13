package proc

import (
	"context"
	"testing"

	"github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestTiFlashInstancePrepare_AllowsLatestAlias(t *testing.T) {
	dir := t.TempDir()
	inst := &TiFlashInstance{
		ProcessInfo: ProcessInfo{
			Dir:        dir,
			Host:       "127.0.0.1",
			StatusPort: 8234,
			BinPath:    "/bin/tiflash",
			Version:    utils.Version(utils.LatestVersionAlias),
			Service:    ServiceTiFlash,
		},
		Plan: TiFlashPlan{
			PDAddrs:         []string{"127.0.0.1:2379"},
			ServicePort:     3930,
			ProxyPort:       20170,
			ProxyStatusPort: 20292,
		},
	}

	require.NoError(t, inst.Prepare(context.Background()))
}

func TestTiFlashInstancePrepare_RejectsOldSemver(t *testing.T) {
	inst := &TiFlashInstance{
		ProcessInfo: ProcessInfo{
			Version: utils.Version("v6.5.0"),
			Service: ServiceTiFlash,
		},
	}

	require.Error(t, inst.Prepare(context.Background()))
}
