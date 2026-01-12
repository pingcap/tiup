package proc

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	// ServiceDMMaster is the service ID for DM-master.
	ServiceDMMaster ServiceID = "dm-master"
	// ServiceDMWorker is the service ID for DM-worker.
	ServiceDMWorker ServiceID = "dm-worker"

	// ComponentDMMaster is the repository component ID for DM-master.
	ComponentDMMaster RepoComponentID = "dm-master"
	// ComponentDMWorker is the repository component ID for DM-worker.
	ComponentDMWorker RepoComponentID = "dm-worker"
)

// DMMasterPlan is the service-specific plan for DM-master.
type DMMasterPlan struct {
	InitialCluster []DMMemberPlan
	RequireReady   bool
}

// DMMemberPlan is one member in the DM initial cluster.
type DMMemberPlan struct {
	Name       string
	PeerAddr   string // host:peerPort
	MasterAddr string // host:statusPort
}

// DMWorkerPlan is the service-specific plan for DM-worker.
type DMWorkerPlan struct {
	MasterAddrs []string // host:statusPort
}

// DMMaster represent a DM master instance.
type DMMaster struct {
	ProcessInfo
	Plan DMMasterPlan
}

var _ Process = &DMMaster{}
var _ ReadyWaiter = &DMMaster{}

// DMWorker represent a DM worker instance.
type DMWorker struct {
	ProcessInfo
	Plan DMWorkerPlan
}

var _ Process = &DMWorker{}

func init() {
	RegisterComponentDisplayName(ComponentDMMaster, "DM-master")
	RegisterServiceDisplayName(ServiceDMMaster, "DM-master")
	RegisterComponentDisplayName(ComponentDMWorker, "DM-worker")
	RegisterServiceDisplayName(ServiceDMWorker, "DM-worker")

	registerPlannedProcessFactory(ServiceDMMaster, func(plan ServicePlan, info ProcessInfo, _ SharedOptions, _ string) (Process, error) {
		if plan.DMMaster == nil {
			name := info.Name()
			if name == "" {
				name = ServiceDMMaster.String()
			}
			return nil, errors.Errorf("missing dm-master plan for %s", name)
		}
		return &DMMaster{Plan: *plan.DMMaster, ProcessInfo: info}, nil
	})
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

// Prepare builds the DM-master process command.
func (m *DMMaster) Prepare(ctx context.Context) error {
	info := m.Info()
	advertiseMaster := utils.JoinHostPort(AdvertiseHost(m.Host), m.StatusPort)
	advertisePeer := utils.JoinHostPort(AdvertiseHost(m.Host), m.Port)
	selfName := ""
	if info != nil {
		selfName = info.Name()
	}
	if selfName != "" {
		for _, member := range m.Plan.InitialCluster {
			if member.Name != selfName {
				continue
			}
			if member.MasterAddr != "" {
				advertiseMaster = member.MasterAddr
			}
			if member.PeerAddr != "" {
				advertisePeer = member.PeerAddr
			}
			break
		}
	}

	args := []string{
		fmt.Sprintf("--name=%s", info.Name()),
		fmt.Sprintf("--master-addr=http://%s", utils.JoinHostPort(m.Host, m.StatusPort)),
		fmt.Sprintf("--advertise-addr=http://%s", advertiseMaster),
		fmt.Sprintf("--peer-urls=http://%s", utils.JoinHostPort(m.Host, m.Port)),
		fmt.Sprintf("--advertise-peer-urls=http://%s", advertisePeer),
		fmt.Sprintf("--log-file=%s", m.LogFile()),
	}

	endpoints := make([]string, 0, len(m.Plan.InitialCluster))
	for _, member := range m.Plan.InitialCluster {
		if member.Name == "" || member.PeerAddr == "" {
			continue
		}
		endpoints = append(endpoints, fmt.Sprintf("%s=http://%s", member.Name, member.PeerAddr))
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
	info := m.Info()
	if info != nil {
		name := info.Name()
		if name != "" {
			for _, member := range m.Plan.InitialCluster {
				if member.Name == name && member.MasterAddr != "" {
					return member.MasterAddr
				}
			}
		}
	}
	return utils.JoinHostPort(AdvertiseHost(m.Host), m.StatusPort)
}

// WaitReady implements ReadyWaiter.
//
// DM-master is considered ready when it is active (or leader) in the DM cluster.
func (m *DMMaster) WaitReady(ctx context.Context) error {
	if m == nil || !m.Plan.RequireReady {
		return nil
	}

	ctx, cancel := withTimeoutSeconds(ctx, m.UpTimeout)
	defer cancel()

	addrs := make([]string, 0, len(m.Plan.InitialCluster))
	for _, member := range m.Plan.InitialCluster {
		if member.MasterAddr != "" {
			addrs = append(addrs, member.MasterAddr)
		}
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
		if state := cmd.ProcessState; state != nil {
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
