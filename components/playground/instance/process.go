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
)

// ErrorWaitTimeout is used to represent timeout of a command
// Example:
//		_ = syscall.Kill(cmd.Process.Pid, syscall.SIGKILL)
//		if err := WaitContext(context.WithTimeout(context.Background(), 3), cmd); err == ErrorWaitTimeout {
//			// Do something
//		}
var ErrorWaitTimeout = errors.New("wait command timeout")

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
	return WaitContext(ctx, p.cmd)
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

// WaitContext wrap cmd.Wait with context
func WaitContext(ctx context.Context, cmd *exec.Cmd) error {
	// We use cmd.Process.Wait instead of cmd.Wait because cmd.Wait is not reenterable
	c := make(chan error, 1)
	go func() {
		if cmd == nil || cmd.Process == nil {
			c <- nil
		} else {
			_, err := cmd.Process.Wait()
			c <- err
		}
	}()
	select {
	case <-ctx.Done():
		return ErrorWaitTimeout
	case err := <-c:
		return err
	}
}
