package instance

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pingcap/tiup/pkg/utils"
)

// DMWorker represent a DM worker instance.
type DMWorker struct {
	instance
	masters []*DMMaster
}

var _ Instance = &DMWorker{}

// NewDMWorker create a DMWorker instance.
func NewDMWorker(shOpt SharedOptions, binPath string, dir, host, configPath string, id int, port int, masters []*DMMaster) *DMWorker {
	if port <= 0 {
		port = 8262
	}
	return &DMWorker{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Role:       "dm-worker",
			Port:       utils.MustGetFreePort(host, port, shOpt.PortOffset),
			ConfigPath: configPath,
		},
		masters: masters,
	}
}

// MasterAddrs return the master addresses.
func (w *DMWorker) MasterAddrs() []string {
	var addrs []string
	for _, master := range w.masters {
		addrs = append(addrs, utils.JoinHostPort(AdvertiseHost(master.Host), master.StatusPort))
	}
	return addrs
}

// Start starts the instance.
func (w *DMWorker) Start(ctx context.Context) error {
	args := []string{
		fmt.Sprintf("--name=%s", w.Name()),
		fmt.Sprintf("--worker-addr=%s", utils.JoinHostPort(w.Host, w.Port)),
		fmt.Sprintf("--advertise-addr=%s", utils.JoinHostPort(AdvertiseHost(w.Host), w.Port)),
		fmt.Sprintf("--join=%s", strings.Join(w.MasterAddrs(), ",")),
		fmt.Sprintf("--log-file=%s", w.LogFile()),
	}

	if w.ConfigPath != "" {
		args = append(args, fmt.Sprintf("--config=%s", w.ConfigPath))
	}

	return w.PrepareProcess(ctx, w.BinPath, args, nil, w.Dir)
}

// LogFile return the log file of the instance.
func (w *DMWorker) LogFile() string {
	return filepath.Join(w.Dir, "dm-worker.log")
}
