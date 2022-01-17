package instance

import (
	"context"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/environment"
	tiupexec "github.com/pingcap/tiup/pkg/exec"
	"github.com/pingcap/tiup/pkg/utils"
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

// NewComponentProcessWithEnvs create a Process instance with given environment variables.
func NewComponentProcessWithEnvs(ctx context.Context, dir, binPath, component string, version utils.Version, envs []string, arg ...string) (Process, error) {
	if dir == "" {
		panic("dir must be set")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	env := environment.GlobalEnv()
	params := &tiupexec.PrepareCommandParams{
		Ctx:         ctx,
		Component:   component,
		Version:     version,
		BinPath:     binPath,
		InstanceDir: dir,
		Args:        arg,
		Env:         env,
	}
	cmd, err := PrepareCommandForPlayground(params, dir)
	if err != nil {
		return nil, err
	}
	cmd.Env = append(cmd.Env, envs...)

	return &process{cmd: cmd}, nil
}

// NewComponentProcess create a Process instance.
func NewComponentProcess(ctx context.Context, dir, binPath, component string, version utils.Version, arg ...string) (Process, error) {
	return NewComponentProcessWithEnvs(ctx, dir, binPath, component, version, nil, arg...)
}

// PrepareCommandForPlayground call PrepareCommand and set SysProcAttr
func PrepareCommandForPlayground(p *tiupexec.PrepareCommandParams, dir string) (*exec.Cmd, error) {
	cmd, err := tiupexec.PrepareCommand(p)
	if err != nil {
		return nil, err
	}
	cmd.Dir = dir
	cmd.SysProcAttr = SysProcAttr
	return cmd, nil
}
