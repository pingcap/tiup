package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/pingcap/tiup/components/playground-ng/proc"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestHelperProcess_ExitWithCode(t *testing.T) {
	if os.Getenv("TIUP_PLAYGROUND_HELPER_PROCESS") != "1" {
		return
	}
	for i := 0; i < len(os.Args); i++ {
		if os.Args[i] != "--" || i+1 >= len(os.Args) {
			continue
		}
		code, err := strconv.Atoi(os.Args[i+1])
		if err != nil {
			os.Exit(2)
		}
		os.Exit(code)
	}
	os.Exit(1)
}

func TestHandleDisplay_JSON_VerboseStatusAndFields(t *testing.T) {
	makeProc := func(serviceID proc.ServiceID, id, pid int, status string) proc.Process {
		info := &proc.ProcessInfo{
			Service:         serviceID,
			ID:              id,
			RepoComponentID: proc.RepoComponentID(serviceID),
			Version:         tiuputils.Version("v7.5.0"),
			BinPath:         "/tmp/bin",
		}
		if pid > 0 {
			cmd := &exec.Cmd{Process: &os.Process{Pid: pid}}
			if status == "exited" {
				exitCmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess_ExitWithCode", "--", "3")
				exitCmd.Env = append(os.Environ(), "TIUP_PLAYGROUND_HELPER_PROCESS=1")
				_ = exitCmd.Run()
				cmd = exitCmd
				if exitCmd.Process != nil {
					pid = exitCmd.Process.Pid
				}
			}
			info.Proc = &displayOSProcess{pid: pid, cmd: cmd, uptime: "1s"}
		}
		base := &displayProcess{info: info, logFile: "/tmp/log"}
		return &displayAddrProcess{displayProcess: base, addr: "127.0.0.1:1234"}
	}

	state := &controllerState{
		procs: map[proc.ServiceID][]proc.Process{
			"svc-a": {makeProc("svc-a", 0, 0, "")},
			"svc-b": {makeProc("svc-b", 0, 111, "running")},
			"svc-c": {makeProc("svc-c", 0, 222, "exited")},
		},
	}
	pg := NewPlayground(t.TempDir(), 0)

	var buf bytes.Buffer
	require.NoError(t, pg.handleDisplay(state, &buf, true, true))

	var items []displayItem
	require.NoError(t, json.Unmarshal(buf.Bytes(), &items))
	require.Len(t, items, 3)

	require.Equal(t, "svc-a", items[0].ServiceID)
	require.Equal(t, "not started", items[0].Status)

	require.Equal(t, "svc-b", items[1].ServiceID)
	require.Equal(t, "running", items[1].Status)
	require.Greater(t, items[1].PID, 0)

	require.Equal(t, "svc-c", items[2].ServiceID)
	require.Equal(t, "exited(3)", items[2].Status)
}

func TestPrettifyUserPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skipf("os.UserHomeDir unavailable: %v", err)
	}

	require.Equal(t, "", prettifyUserPath(""))
	require.Equal(t, "~", prettifyUserPath(home))
	nested := filepath.Join(home, "a", "b")
	wantNested := "~" + string(os.PathSeparator) + "a" + string(os.PathSeparator) + "b"
	require.Equal(t, wantNested, prettifyUserPath(nested))
	require.Equal(t, "/tmp/a", prettifyUserPath("/tmp/a"))
}

type displayAddrProcess struct {
	*displayProcess
	addr string
}

func (p *displayAddrProcess) Addr() string { return p.addr }

type displayOSProcess struct {
	pid    int
	cmd    *exec.Cmd
	uptime string
}

func (p *displayOSProcess) Start() error                     { return nil }
func (p *displayOSProcess) Wait() error                      { return nil }
func (p *displayOSProcess) Pid() int                         { return p.pid }
func (p *displayOSProcess) Uptime() string                   { return p.uptime }
func (p *displayOSProcess) SetOutputFile(fname string) error { return nil }
func (p *displayOSProcess) Cmd() *exec.Cmd                   { return p.cmd }

type displayProcess struct {
	info    *proc.ProcessInfo
	logFile string
}

func (p *displayProcess) Info() *proc.ProcessInfo           { return p.info }
func (p *displayProcess) Prepare(ctx context.Context) error { return nil }
func (p *displayProcess) LogFile() string                   { return p.logFile }
