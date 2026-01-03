package proc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pingcap/errors"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	ServiceGrafana ServiceID = "grafana"

	ComponentGrafana RepoComponentID = "grafana"
)

func init() {
	RegisterComponentDisplayName(ComponentGrafana, "Grafana")
	RegisterServiceDisplayName(ServiceGrafana, "Grafana")
}

type GrafanaInstance struct {
	ProcessInfo

	PrometheusURL string
}

var _ Process = &GrafanaInstance{}

func (inst *GrafanaInstance) LogFile() string {
	return filepath.Join(inst.Dir, "grafana.log")
}

const grafanaClusterName = "Test-Cluster"

func writeDatasourceConfig(fname string, clusterName string, p8sURL string) error {
	if err := utils.MkdirAll(filepath.Dir(fname), 0755); err != nil {
		return errors.AddStack(err)
	}

	tpl := `apiVersion: 1
datasources:
  - name: %s
    type: prometheus
    access: proxy
    url: %s
    withCredentials: false
    isDefault: false
    tlsAuth: false
    tlsAuthWithCACert: false
    version: 1
    editable: true
`

	s := fmt.Sprintf(tpl, clusterName, p8sURL)
	if err := utils.WriteFile(fname, []byte(s), 0644); err != nil {
		return errors.AddStack(err)
	}
	return nil
}

// replaceDatasource replaces dashboard datasource placeholders with the one we
// are using.
// ref: templates/scripts/run_grafana.sh.tpl
func replaceDatasource(dashboardDir string, datasourceName string) error {
	// for "s/\${DS_.*-CLUSTER}/datasourceName/g"
	re := regexp.MustCompile(`\${DS_.*-CLUSTER}`)

	return filepath.Walk(dashboardDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logprinter.Warnf("skip scan %s failed: %v", path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return errors.AddStack(err)
		}

		s := string(data)
		s = strings.ReplaceAll(s, "test-cluster", datasourceName)
		s = strings.ReplaceAll(s, "Test-Cluster", datasourceName)
		s = strings.ReplaceAll(s, "${DS_LIGHTNING}", datasourceName)
		s = re.ReplaceAllLiteralString(s, datasourceName)

		return utils.WriteFile(path, []byte(s), 0644)
	})
}

func writeDashboardConfig(fname string, clusterName string, dir string) error {
	if err := utils.MkdirAll(filepath.Dir(fname), 0755); err != nil {
		return errors.AddStack(err)
	}

	tpl := `apiVersion: 1
providers:
  - name: %s
    folder: %s
    type: file
    disableDeletion: false
    allowUiUpdates: true
    editable: true
    updateIntervalSeconds: 30
    options:
      path: %s
`
	s := fmt.Sprintf(tpl, clusterName, clusterName, dir)
	if err := utils.WriteFile(fname, []byte(s), 0644); err != nil {
		return errors.AddStack(err)
	}
	return nil
}

func resolveGrafanaHome(binPath string) string {
	if binPath == "" {
		return ""
	}

	// Best-effort resolve symlinks so we can walk the real install directory.
	// If it fails, keep the original path.
	if resolved, err := filepath.EvalSymlinks(binPath); err == nil && resolved != "" {
		binPath = resolved
	}

	dir := binPath
	if st, err := os.Stat(binPath); err == nil && !st.IsDir() {
		dir = filepath.Dir(binPath)
	}

	for i := 0; i < 8; i++ {
		defaults := filepath.Join(dir, "conf", "defaults.ini")
		if st, err := os.Stat(defaults); err == nil && !st.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func copyGrafanaDashboards(home, dashboardDir string) error {
	entries, err := os.ReadDir(home)
	if err != nil {
		return errors.AddStack(err)
	}
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(home, name))
		if err != nil {
			return errors.AddStack(err)
		}
		if err := utils.WriteFile(filepath.Join(dashboardDir, name), data, 0644); err != nil {
			return errors.AddStack(err)
		}
	}
	return nil
}

func (inst *GrafanaInstance) Prepare(ctx context.Context) error {
	if inst == nil {
		return errors.New("grafana instance is nil")
	}
	info := inst.Info()
	if inst.Dir == "" {
		return errors.New("grafana dir is empty")
	}
	if inst.BinPath == "" {
		return errors.New("grafana binary not resolved")
	}
	if err := utils.MkdirAll(inst.Dir, 0755); err != nil {
		return errors.AddStack(err)
	}

	home := resolveGrafanaHome(inst.BinPath)
	if home == "" {
		return errors.New("cannot resolve grafana home")
	}

	dashboardDir := filepath.Join(inst.Dir, "dashboards")
	if err := utils.MkdirAll(dashboardDir, 0755); err != nil {
		return errors.AddStack(err)
	}
	if err := copyGrafanaDashboards(home, dashboardDir); err != nil {
		return err
	}
	if err := replaceDatasource(dashboardDir, grafanaClusterName); err != nil {
		return err
	}

	provisioningDir := filepath.Join(inst.Dir, "conf", "provisioning")
	for _, sub := range []string{"dashboards", "datasources", "plugins", "notifiers"} {
		if err := utils.MkdirAll(filepath.Join(provisioningDir, sub), 0755); err != nil {
			return errors.AddStack(err)
		}
	}

	if err := writeDashboardConfig(
		filepath.Join(provisioningDir, "dashboards", "dashboard.yml"),
		grafanaClusterName,
		dashboardDir,
	); err != nil {
		return err
	}

	if err := writeDatasourceConfig(
		filepath.Join(provisioningDir, "datasources", "datasource.yml"),
		grafanaClusterName,
		inst.PrometheusURL,
	); err != nil {
		return err
	}

	custom := fmt.Sprintf(`
[server]
# The ip address to bind to, empty will bind to all interfaces
http_addr = %s

# The http port to use
http_port = %d
`, inst.Host, inst.Port)

	if err := utils.MkdirAll(filepath.Join(inst.Dir, "conf"), 0755); err != nil {
		return errors.AddStack(err)
	}
	customFile := filepath.Join(inst.Dir, "conf", "custom.ini")
	if err := utils.WriteFile(customFile, []byte(custom), 0644); err != nil {
		return errors.AddStack(err)
	}

	args := []string{
		"--homepath", home,
		"--config", customFile,
		fmt.Sprintf("cfg:default.paths.data=%s", filepath.Join(inst.Dir, "data")),
		fmt.Sprintf("cfg:default.paths.logs=%s", filepath.Join(inst.Dir, "log")),
		fmt.Sprintf("cfg:default.paths.plugins=%s", filepath.Join(inst.Dir, "plugins")),
		fmt.Sprintf("cfg:default.paths.provisioning=%s", filepath.Join(inst.Dir, "conf", "provisioning")),
	}
	info.Proc = &cmdProcess{cmd: PrepareCommand(ctx, inst.BinPath, args, nil, home)}
	return nil
}
