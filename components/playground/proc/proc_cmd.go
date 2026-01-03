package proc

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/pingcap/errors"
)

var (
	errNotUp = errors.New("not up")
)

// OSProcess represents an operating system process started by playground.
type OSProcess interface {
	Start() error
	Wait() error
	Pid() int
	Uptime() string
	SetOutputFile(fname string) error
	Cmd() *exec.Cmd
}

// cmdProcess implements OSProcess via an *exec.Cmd.
type cmdProcess struct {
	cmd       *exec.Cmd
	startTime time.Time
	endTime   time.Time

	waitOnce sync.Once
	waitErr  error

	outFile *os.File
}

// Start the process
func (p *cmdProcess) Start() error {
	if p == nil {
		return errNotUp
	}

	// fmt.Printf("Starting `%s`: %s", filepath.Base(p.cmd.Path), strings.Join(p.cmd.Args, " "))
	p.startTime = time.Now()
	p.endTime = time.Time{}
	if err := p.cmd.Start(); err != nil {
		if p.outFile != nil {
			_ = p.outFile.Close()
			p.outFile = nil
		}
		return err
	}
	return nil
}

// Wait implements OSProcess.
func (p *cmdProcess) Wait() error {
	if p == nil {
		return errNotUp
	}

	p.waitOnce.Do(func() {
		p.waitErr = p.cmd.Wait()
		p.endTime = time.Now()
		if p.outFile != nil {
			_ = p.outFile.Close()
			p.outFile = nil
		}
	})

	return p.waitErr
}

// Pid implements OSProcess.
func (p *cmdProcess) Pid() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// Uptime implements OSProcess.
func (p *cmdProcess) Uptime() string {
	if p == nil {
		return "not started"
	}

	if p.startTime.IsZero() {
		return "not started"
	}

	end := p.endTime
	if end.IsZero() {
		end = time.Now()
	}
	duration := end.Sub(p.startTime)
	return duration.String()
}

func (p *cmdProcess) SetOutputFile(fname string) error {
	if p == nil {
		return errNotUp
	}

	if fname == "" {
		return errors.New("output file path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(fname), 0o755); err != nil {
		return errors.AddStack(err)
	}

	f, err := os.OpenFile(fname, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return errors.AddStack(err)
	}
	if p.outFile != nil {
		_ = p.outFile.Close()
	}
	p.outFile = f
	p.setOutput(f)
	return nil
}

func (p *cmdProcess) setOutput(w io.Writer) {
	if p == nil {
		return
	}

	p.cmd.Stdout = w
	p.cmd.Stderr = w
}

func (p *cmdProcess) Cmd() *exec.Cmd {
	if p == nil {
		return nil
	}

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
