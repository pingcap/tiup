package instance

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/localdata"
)

// Process represent process to be run by playground.
type Process struct {
	cmd       *exec.Cmd
	startTime time.Time
}

// Start the process
func (p *Process) Start() error {
	// fmt.Printf("Starting `%s`: %s", filepath.Base(p.cmd.Path), strings.Join(p.cmd.Args, " "))
	p.startTime = time.Now()
	return p.cmd.Start()
}

// Wait implements Instance interface.
func (p *Process) Wait() error {
	return p.cmd.Wait()
}

// Pid implements Instance interface.
func (p *Process) Pid() int {
	return p.cmd.Process.Pid
}

// Uptime implements Instance interface.
func (p *Process) Uptime() string {
	s := p.cmd.ProcessState
	if s != nil && s.Exited() {
		return "exited"
	}

	duration := time.Since(p.startTime)
	return duration.String()
}

func (p *Process) setOutputFile(fname string) error {
	f, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return errors.AddStack(err)
	}
	p.setOutput(f)
	return nil
}

func (p *Process) setOutput(w io.Writer) {
	p.cmd.Stdout = w
	p.cmd.Stderr = w
}

// NewProcess create a Process instance.
func NewProcess(ctx context.Context, dir string, name string, arg ...string) *Process {
	if dir == "" {
		panic("dir must be set")
	}

	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", localdata.EnvNameInstanceDataDir, dir),
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return &Process{
		cmd: cmd,
	}
}
