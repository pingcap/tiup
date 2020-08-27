package spec

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/stretchr/testify/assert"
)

func TestLocalDashboards(t *testing.T) {
	deployDir, err := ioutil.TempDir("", "tiup-*")
	assert.Nil(t, err)
	defer os.RemoveAll(deployDir)
	localDir, err := filepath.Abs("./testdata/dashboards")
	assert.Nil(t, err)

	topo := new(Specification)
	topo.Grafana = append(topo.Grafana, GrafanaSpec{
		Host:         "127.0.0.1",
		Port:         3000,
		DashboardDir: localDir,
	})

	comp := GrafanaComponent{topo}
	ints := comp.Instances()

	assert.Equal(t, len(ints), 1)
	grafanaInstance := ints[0].(*GrafanaInstance)

	clusterName := "tiup-test-cluster-" + uuid.New().String()
	e := &executor.Local{}
	err = grafanaInstance.initDashboards(e, topo.Grafana[0], meta.DirPaths{Deploy: deployDir}, clusterName)
	assert.Nil(t, err)

	assert.FileExists(t, path.Join(deployDir, "dashboards", "tidb.json"))
	fs, err := ioutil.ReadDir(localDir)
	assert.Nil(t, err)
	for _, f := range fs {
		assert.FileExists(t, path.Join(deployDir, "dashboards", f.Name()))
	}
}
