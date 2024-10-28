package instance

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/utils"
)

type DMWorker struct {
	instance
	Process

	masters []*DMMaster
}

var _ Instance = &DMWorker{}

// NewDMWorker create a DMWorker instance.
func NewDMWorker(binPath string, dir, host, configPath string, portOffset int, id int, port int, masters []*DMMaster) *DMWorker {
	if port <= 0 {
		port = 8262
	}
	return &DMWorker{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, port, portOffset),
			ConfigPath: configPath,
		},
		masters: masters,
	}
}

func (w *DMWorker) MasterAddrs() []string {
	var addrs []string
	for _, master := range w.masters {
		addrs = append(addrs, utils.JoinHostPort(AdvertiseHost(master.Host), master.StatusPort))
	}
	return addrs
}

func (w *DMWorker) Name() string {
	return fmt.Sprintf("dm-worker-%d", w.ID)
}

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

	w.Process = &process{cmd: PrepareCommand(ctx, w.BinPath, args, nil, w.Dir)}

	logIfErr(w.Process.SetOutputFile(w.LogFile()))

	// try to wait for the master to be ready
	// this is a very ugly implementation, but it may mostly works
	// TODO: find a better way to do this,
	// e.g, let master support a HTTP API to check if it's ready
	time.Sleep(time.Second * 3)
	return w.Process.Start()
}

func (w *DMWorker) Component() string {
	return "dm-worker"
}

func (w *DMWorker) LogFile() string {
	return filepath.Join(w.Dir, "dm-worker.log")
}
