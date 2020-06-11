package instance

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"github.com/pingcap/tiup/pkg/utils"
)

// Pump represent a pump instance.
type Pump struct {
	instance
	pds []*PDInstance
	cmd *exec.Cmd
}

var _ Instance = &Pump{}

// NewPump create a Pump instance.
func NewPump(binPath string, dir, host, configPath string, id int, pds []*PDInstance) *Pump {
	return &Pump{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, 8249),
			ConfigPath: configPath,
		},
		pds: pds,
	}
}

// NodeID return the node id of pump.
func (p *Pump) NodeID() string {
	return fmt.Sprintf("pump_%d", p.ID)
}

// Ready return nil when pump is ready to serve.
func (p *Pump) Ready(ctx context.Context) error {
	url := fmt.Sprintf("http://%s:%d/status", p.Host, p.Port)

	for {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			// just retry
		}
	}
}

// Addr return the address of Pump.
func (p *Pump) Addr() string {
	return fmt.Sprintf("%s:%d", advertiseHost(p.Host), p.Port)
}

// Start implements Instance interface.
func (p *Pump) Start(ctx context.Context, version v0manifest.Version) error {
	if err := os.MkdirAll(p.Dir, 0755); err != nil {
		return err
	}

	var urls []string
	for _, pd := range p.pds {
		urls = append(urls, fmt.Sprintf("http://%s:%d", pd.Host, pd.StatusPort))
	}

	args := []string{
		"tiup", fmt.Sprintf("--binpath=%s", p.BinPath),
		CompVersion("pump", version),
		fmt.Sprintf("--node-id=%s", p.NodeID()),
		fmt.Sprintf("--addr=%s:%d", p.Host, p.Port),
		fmt.Sprintf("--advertise-addr=%s:%d", advertiseHost(p.Host), p.Port),
		fmt.Sprintf("--pd-urls=%s", strings.Join(urls, ",")),
		fmt.Sprintf("--log-file=%s", filepath.Join(p.Dir, "pump.log")),
	}
	if p.ConfigPath != "" {
		args = append(args, fmt.Sprintf("--config=%s", p.ConfigPath))
	}

	p.cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	p.cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", localdata.EnvNameInstanceDataDir, p.Dir),
	)
	p.cmd.Stderr = os.Stderr
	p.cmd.Stdout = os.Stdout

	return p.cmd.Start()
}

// Wait implements Instance interface.
func (p *Pump) Wait() error {
	return p.cmd.Wait()
}

// Pid implements Instance interface.
func (p *Pump) Pid() int {
	return p.cmd.Process.Pid
}
