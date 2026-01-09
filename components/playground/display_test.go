package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
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
			info.Proc = &stubOSProcess{pid: pid, cmd: cmd, uptime: "1s"}
		}
		base := &stubProcess{info: info, logFile: "/tmp/log"}
		return &stubAddrProcess{stubProcess: base, addr: "127.0.0.1:1234"}
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
	if err := pg.handleDisplay(state, &buf, true, true); err != nil {
		t.Fatalf("handleDisplay: %v", err)
	}

	var items []displayItem
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("unexpected items length: %d", len(items))
	}

	if items[0].ServiceID != "svc-a" || items[0].Status != "not started" {
		t.Fatalf("unexpected item[0]: %+v", items[0])
	}
	if items[1].ServiceID != "svc-b" || items[1].Status != "running" || items[1].PID <= 0 {
		t.Fatalf("unexpected item[1]: %+v", items[1])
	}
	if items[2].ServiceID != "svc-c" || items[2].Status != "exited(3)" {
		t.Fatalf("unexpected item[2]: %+v", items[2])
	}
}

func TestPrettifyUserPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skipf("os.UserHomeDir unavailable: %v", err)
	}

	if got := prettifyUserPath(""); got != "" {
		t.Fatalf("unexpected empty path: %q", got)
	}
	if got := prettifyUserPath(home); got != "~" {
		t.Fatalf("unexpected home path: %q", got)
	}
	nested := filepath.Join(home, "a", "b")
	wantNested := "~" + string(os.PathSeparator) + "a" + string(os.PathSeparator) + "b"
	if got := prettifyUserPath(nested); got != wantNested {
		t.Fatalf("unexpected nested path: %q", got)
	}
	if got := prettifyUserPath("/tmp/a"); got != "/tmp/a" {
		t.Fatalf("unexpected other path: %q", got)
	}
}
