package proc

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	// ServiceDMMaster is the service ID for DM-master.
	ServiceDMMaster ServiceID = "dm-master"

	// ComponentDMMaster is the repository component ID for DM-master.
	ComponentDMMaster RepoComponentID = "dm-master"
)

// DMMaster represent a DM master instance.
type DMMaster struct {
	ProcessInfo
	InitEndpoints []*DMMaster
	RequireReady  bool
}

var _ Process = &DMMaster{}
var _ ReadyWaiter = &DMMaster{}

func init() {
	RegisterComponentDisplayName(ComponentDMMaster, "DM-master")
	RegisterServiceDisplayName(ServiceDMMaster, "DM-master")
}

// Prepare builds the DM-master process command.
func (m *DMMaster) Prepare(ctx context.Context) error {
	info := m.Info()
	args := []string{
		fmt.Sprintf("--name=%s", info.Name()),
		fmt.Sprintf("--master-addr=http://%s", utils.JoinHostPort(m.Host, m.StatusPort)),
		fmt.Sprintf("--advertise-addr=http://%s", utils.JoinHostPort(AdvertiseHost(m.Host), m.StatusPort)),
		fmt.Sprintf("--peer-urls=http://%s", utils.JoinHostPort(m.Host, m.Port)),
		fmt.Sprintf("--advertise-peer-urls=http://%s", utils.JoinHostPort(AdvertiseHost(m.Host), m.Port)),
		fmt.Sprintf("--log-file=%s", m.LogFile()),
	}

	endpoints := make([]string, 0)
	for _, master := range m.InitEndpoints {
		endpoints = append(endpoints, fmt.Sprintf("%s=http://%s", master.Info().Name(), utils.JoinHostPort(master.Host, master.Port)))
	}
	args = append(args, fmt.Sprintf("--initial-cluster=%s", strings.Join(endpoints, ",")))

	if m.ConfigPath != "" {
		args = append(args, fmt.Sprintf("--config=%s", m.ConfigPath))
	}

	info.Proc = &cmdProcess{cmd: PrepareCommand(ctx, m.BinPath, args, nil, m.Dir)}
	return nil
}

// LogFile return the log file path of the instance.
func (m *DMMaster) LogFile() string {
	return filepath.Join(m.Dir, "dm-master.log")
}

// Addr return the address of the instance.
func (m *DMMaster) Addr() string {
	return utils.JoinHostPort(m.Host, m.StatusPort)
}

// WaitReady implements ReadyWaiter.
//
// DM-master is considered ready when it is active (or leader) in the DM cluster.
func (m *DMMaster) WaitReady(ctx context.Context) error {
	if m == nil || !m.RequireReady {
		return nil
	}

	ctx, cancel := withTimeoutSeconds(ctx, m.UpTimeout)
	defer cancel()

	addrs := make([]string, 0, len(m.InitEndpoints))
	for _, master := range m.InitEndpoints {
		addrs = append(addrs, master.Addr())
	}
	if len(addrs) == 0 {
		addrs = append(addrs, m.Addr())
	}
	client := api.NewDMMasterClient(addrs, 5*time.Second, nil)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		proc := m.Info().Proc
		if proc == nil {
			return fmt.Errorf("initialize command failed")
		}
		cmd := proc.Cmd()
		if cmd == nil {
			return fmt.Errorf("initialize command failed")
		}
		if state := cmd.ProcessState; state != nil && state.Exited() {
			return fmt.Errorf("process exited with code: %d", state.ExitCode())
		}

		_, isActive, isLeader, err := client.GetMaster(m.Info().Name())
		if err == nil && (isActive || isLeader) {
			return nil
		}

		select {
		case <-ctx.Done():
			err := ctx.Err()
			if err == context.DeadlineExceeded && m.UpTimeout > 0 {
				return readyTimeoutError(m.UpTimeout)
			}
			return err
		case <-ticker.C:
		}
	}
}
