package proc

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pingcap/tiup/pkg/utils"
)

const (
	// ServiceDMWorker is the service ID for DM-worker.
	ServiceDMWorker ServiceID = "dm-worker"

	// ComponentDMWorker is the repository component ID for DM-worker.
	ComponentDMWorker RepoComponentID = "dm-worker"
)

// DMWorker represent a DM worker instance.
type DMWorker struct {
	ProcessInfo
	Masters []*DMMaster
}

var _ Process = &DMWorker{}

func init() {
	RegisterComponentDisplayName(ComponentDMWorker, "DM-worker")
	RegisterServiceDisplayName(ServiceDMWorker, "DM-worker")
}

// MasterAddrs return the master addresses.
func (w *DMWorker) MasterAddrs() []string {
	var addrs []string
	for _, master := range w.Masters {
		addrs = append(addrs, utils.JoinHostPort(AdvertiseHost(master.Host), master.StatusPort))
	}
	return addrs
}

// Prepare builds the DM-worker process command.
func (w *DMWorker) Prepare(ctx context.Context) error {
	info := w.Info()
	args := []string{
		fmt.Sprintf("--name=%s", info.Name()),
		fmt.Sprintf("--worker-addr=%s", utils.JoinHostPort(w.Host, w.Port)),
		fmt.Sprintf("--advertise-addr=%s", utils.JoinHostPort(AdvertiseHost(w.Host), w.Port)),
		fmt.Sprintf("--join=%s", strings.Join(w.MasterAddrs(), ",")),
		fmt.Sprintf("--log-file=%s", w.LogFile()),
	}

	if w.ConfigPath != "" {
		args = append(args, fmt.Sprintf("--config=%s", w.ConfigPath))
	}

	info.Proc = &cmdProcess{cmd: PrepareCommand(ctx, w.BinPath, args, nil, w.Dir)}
	return nil
}

// LogFile return the log file of the instance.
func (w *DMWorker) LogFile() string {
	return filepath.Join(w.Dir, "dm-worker.log")
}
