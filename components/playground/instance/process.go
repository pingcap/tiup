package instance

import (
	"context"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/environment"
	tiupexec "github.com/pingcap/tiup/pkg/exec"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"github.com/pingcap/tiup/pkg/utils"
)

// Process represent process to be run by playground
type Process interface {
	Start() error
	Wait(ctx context.Context) error
	Pid() int
	Uptime() string
	SetOutputFile(fname string) error
	Cmd() *exec.Cmd
}

// process implementes Process
type process struct {
	cmd       *exec.Cmd
	startTime time.Time
}

// Start the process
func (p *process) Start() error {
	// fmt.Printf("Starting `%s`: %s", filepath.Base(p.cmd.Path), strings.Join(p.cmd.Args, " "))
	p.startTime = time.Now()
	return p.cmd.Start()
}

// Wait implements Instance interface.
func (p *process) Wait(ctx context.Context) error {
	return utils.WaitContext(ctx, p.cmd)
}

// Pid implements Instance interface.
func (p *process) Pid() int {
	return p.cmd.Process.Pid
}

// Uptime implements Instance interface.
func (p *process) Uptime() string {
	s := p.cmd.ProcessState
	if s != nil && s.Exited() {
		return "exited"
	}

	duration := time.Since(p.startTime)
	return duration.String()
}

func (p *process) SetOutputFile(fname string) error {
	f, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return errors.AddStack(err)
	}
	p.setOutput(f)
	return nil
}

func (p *process) setOutput(w io.Writer) {
	p.cmd.Stdout = w
	p.cmd.Stderr = w
}

func (p *process) Cmd() *exec.Cmd {
	return p.cmd
}

// NewComponentProcess create a Process instance.
func NewComponentProcess(ctx context.Context, dir, binPath, component string, version v0manifest.Version, arg ...string) (Process, error) {
	if dir == "" {
		panic("dir must be set")
	}

	env := environment.GlobalEnv()
	cmd, err := tiupexec.PrepareCommand(ctx, component, version, binPath, "", dir, arg, env)
	if err != nil {
		return nil, err
	}

	return &process{cmd: cmd}, nil
}
