package module

import (
	"bytes"
	"fmt"
	"time"

	"github.com/pingcap-incubator/tiops/pkg/executor"
	"github.com/pingcap/errors"
)

// WaitForConfig is the configurations of WaitFor module.
type WaitForConfig struct {
	Port  int           // Port number to poll.
	Sleep time.Duration // Duration to sleep between checks, default 1 second.
	// Choices:
	// started
	// stopped
	// When checking a port started will ensure the port is open, stopped will check that it is closed
	State   string
	Timeout time.Duration // Maximum duration to wait for.
}

// WaitFor is the module used to wait for some condition.
type WaitFor struct {
	c WaitForConfig
}

// NewWaitFor create a WaitFor instance.
func NewWaitFor(c WaitForConfig) *WaitFor {
	if c.Sleep == 0 {
		c.Sleep = time.Second
	}
	if c.Timeout == 0 {
		c.Timeout = time.Second * 60
	}
	if c.State == "" {
		c.State = "started"
	}

	w := &WaitFor{
		c: c,
	}

	return w
}

// Execute the module return nil if successfully wait for the event.
func (w *WaitFor) Execute(e executor.TiOpsExecutor) (err error) {
	// netstat -tunlp | grep ":4000 "

	start := time.Now()
	patern := []byte(fmt.Sprintf(":%d ", w.c.Port))

	for {
		stdout, _, err := e.Execute("netstat -tunlp", false)
		if err == nil {
			switch w.c.State {
			case "started":
				if bytes.Contains(stdout, patern) {
					return nil
				}
			case "stopped":
				if !bytes.Contains(stdout, patern) {
					return nil
				}
			}
		}

		if time.Since(start) >= w.c.Timeout {
			return errors.Errorf("timeout to wait for port %s", w.c.State)
		}
		time.Sleep(w.c.Sleep)
	}
}
