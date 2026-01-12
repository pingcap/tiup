package proc

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	// ServiceDMWorker is the service ID for DM-worker.
	ServiceDMWorker ServiceID = "dm-worker"

	// ComponentDMWorker is the repository component ID for DM-worker.
	ComponentDMWorker RepoComponentID = "dm-worker"
)

// DMWorkerPlan is the service-specific plan for DM-worker.
type DMWorkerPlan struct {
	MasterAddrs []string // host:statusPort
}

// DMWorker represent a DM worker instance.
type DMWorker struct {
	ProcessInfo
	Plan DMWorkerPlan
}

var _ Process = &DMWorker{}

func init() {
	RegisterComponentDisplayName(ComponentDMWorker, "DM-worker")
	RegisterServiceDisplayName(ServiceDMWorker, "DM-worker")

	registerPlannedProcessFactory(ServiceDMWorker, func(plan ServicePlan, info ProcessInfo, _ SharedOptions, _ string) (Process, error) {
		if plan.DMWorker == nil {
			name := info.Name()
			if name == "" {
				name = ServiceDMWorker.String()
			}
			return nil, errors.Errorf("missing dm-worker plan for %s", name)
		}
		return &DMWorker{Plan: *plan.DMWorker, ProcessInfo: info}, nil
	})
}

// MasterAddrs return the master addresses.
func (w *DMWorker) MasterAddrs() []string {
	return append([]string(nil), w.Plan.MasterAddrs...)
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
