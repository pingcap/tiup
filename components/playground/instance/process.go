package instance

import (
	"context"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/pingcap/errors"
)

// Process represent process to be run by playground
type Process interface {
	Start() error
	Wait() error
	Pid() int
	Uptime() string
	SetOutputFile(fname string) error
	Cmd() *exec.Cmd
}

// process implements Process
type process struct {
	cmd       *exec.Cmd
	startTime time.Time

	waitOnce sync.Once
	waitErr  error
}

// Start the process
func (p *process) Start() error {
	// fmt.Printf("Starting `%s`: %s", filepath.Base(p.cmd.Path), strings.Join(p.cmd.Args, " "))
	p.startTime = time.Now()
	return p.cmd.Start()
}

// Wait implements Instance interface.
func (p *process) Wait() error {
	p.waitOnce.Do(func() {
		p.waitErr = p.cmd.Wait()
	})

	return p.waitErr
}

// Pid implements Instance interface.
func (p *process) Pid() int {
	return p.cmd.Process.Pid
}

// Uptime implements Instance interface.
func (p *process) Uptime() string {
	s := p.cmd.ProcessState

	if s != nil {
		return s.String()
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

// PrepareCommand return command for playground
func PrepareCommand(ctx context.Context, binPath string, args, envs []string, workDir string) *exec.Cmd {
	c := exec.CommandContext(ctx, binPath, args...)

	c.Env = os.Environ()
	c.Env = append(c.Env, envs...)
	c.Dir = workDir
	c.SysProcAttr = SysProcAttr
	return c
}
