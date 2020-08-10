package executor

import (
	"bytes"
	"context"
	"io/ioutil"
	"os/exec"
	"strings"
	"time"

	"github.com/pingcap/errors"
)

// Local execute the command at local host.
type Local struct {
}

var _ Executor = &Local{}

// Execute implements Executor interface.
func (l *Local) Execute(cmd string, sudo bool, timeout ...time.Duration) (stdout []byte, stderr []byte, err error) {
	ctx := context.Background()
	var cancel context.CancelFunc
	if len(timeout) > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout[0])
		defer cancel()
	}

	args := strings.Split(cmd, " ")
	command := exec.CommandContext(ctx, args[0], args[1:]...)

	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)
	command.Stdout = stdoutBuf
	command.Stderr = stderrBuf

	err = command.Run()
	stdout = stderrBuf.Bytes()
	stderr = stderrBuf.Bytes()
	return
}

// Transfer implements Executer interface.
func (l *Local) Transfer(src string, dst string, download bool) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return errors.AddStack(err)
	}

	err = ioutil.WriteFile(dst, data, 0644)
	if err != nil {
		return errors.AddStack(err)
	}

	return nil
}
