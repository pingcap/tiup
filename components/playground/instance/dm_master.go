package instance

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pingcap/tiup/pkg/utils"
)

type DMMaster struct {
	instance
	Process
	initEndpoints []*DMMaster
}

var _ Instance = &DMMaster{}

func NewDMMaster(binPath string, dir, host, configPath string, portOffset int, id int, port int) *DMMaster {
	if port <= 0 {
		port = 8261
	}
	return &DMMaster{
		instance: instance{
			BinPath: binPath,
			ID:      id,
			Dir:     dir,
			Host:    host,
			Port:    utils.MustGetFreePort(host, 8291, portOffset),
			// Similar like PD's client port, here use StatusPort for Master Port.
			StatusPort: utils.MustGetFreePort(host, port, portOffset),
			ConfigPath: configPath,
		},
	}
}

func (m *DMMaster) Name() string {
	return fmt.Sprintf("dm-master-%d", m.ID)
}

func (m *DMMaster) Start(ctx context.Context) error {
	args := []string{
		fmt.Sprintf("--name=%s", m.Name()),
		fmt.Sprintf("--master-addr=http://%s", utils.JoinHostPort(m.Host, m.StatusPort)),
		fmt.Sprintf("--advertise-addr=http://%s", utils.JoinHostPort(AdvertiseHost(m.Host), m.StatusPort)),
		fmt.Sprintf("--peer-urls=http://%s", utils.JoinHostPort(m.Host, m.Port)),
		fmt.Sprintf("--advertise-peer-urls=http://%s", utils.JoinHostPort(AdvertiseHost(m.Host), m.Port)),
		fmt.Sprintf("--log-file=%s", m.LogFile()),
	}

	endpoints := make([]string, 0)
	for _, master := range m.initEndpoints {
		endpoints = append(endpoints, fmt.Sprintf("%s=http://%s", master.Name(), utils.JoinHostPort(master.Host, master.Port)))
	}
	args = append(args, fmt.Sprintf("--initial-cluster=%s", strings.Join(endpoints, ",")))

	if m.ConfigPath != "" {
		args = append(args, fmt.Sprintf("--config=%s", m.ConfigPath))
	}

	m.Process = &process{cmd: PrepareCommand(ctx, m.BinPath, args, nil, m.Dir)}

	logIfErr(m.Process.SetOutputFile(m.LogFile()))
	return m.Process.Start()
}

func (m *DMMaster) SetInitEndpoints(endpoints []*DMMaster) {
	m.initEndpoints = endpoints
}

func (m *DMMaster) Component() string {
	return "dm-master"
}

func (m *DMMaster) LogFile() string {
	return filepath.Join(m.Dir, "dm-master.log")
}
