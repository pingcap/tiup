package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/stretchr/testify/require"
)

func TestExecute_PS_IgnoresTemporaryTIUPInstanceDataDir(t *testing.T) {
	tiupHome := t.TempDir()
	t.Setenv(localdata.EnvNameHome, tiupHome)

	// Mimic the TiUP runner creating an empty, auto-generated instance dir for
	// this invocation (when users don't specify a global --tag).
	scratchDir := filepath.Join(tiupHome, localdata.DataParentDir, "V8CMwY9")
	require.NoError(t, os.MkdirAll(scratchDir, 0o755))
	t.Setenv(localdata.EnvNameInstanceDataDir, scratchDir)

	dataDir := filepath.Join(tiupHome, localdata.DataParentDir, "real")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))

	startedAt := time.Date(2026, 1, 13, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	pidBody := "pid=" + strconv.Itoa(os.Getpid()) + "\nstarted_at=" + startedAt + "\ntag=real\n"
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, playgroundPIDFileName), []byte(pidBody), 0o644))

	items := []displayItem{
		{Name: "pd-0", ServiceID: "pd", Status: "running", Version: "v8.5.4"},
		{Name: "tidb-0", ServiceID: "tidb", Status: "running", Version: "v8.5.4"},
		{Name: "tikv-0", ServiceID: "tikv", Status: "running", Version: "v8.5.4"},
	}
	itemsJSON, err := json.Marshal(items)
	require.NoError(t, err)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/command" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "method not allowed"})
			return
		case http.MethodPost:
			var cmd Command
			if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: err.Error()})
				return
			}
			if cmd.Type != DisplayCommandType {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "unexpected command"})
				return
			}
			_ = json.NewEncoder(w).Encode(CommandReply{OK: true, Message: string(itemsJSON)})
			return
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "method not allowed"})
			return
		}
	}))
	t.Cleanup(s.Close)

	u, err := url.Parse(s.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	require.NoError(t, dumpPort(filepath.Join(dataDir, playgroundPortFileName), port))

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"tiup-playground-ng", "ps"}

	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() { _ = w.Close() })

	oldStdout := os.Stdout
	t.Cleanup(func() { os.Stdout = oldStdout })
	os.Stdout = w

	state := newCLIState()
	require.NoError(t, execute(state))

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Contains(t, string(out), "real")
	require.False(t, strings.Contains(string(out), "V8CMwY9"), "should not treat scratch instance dir as target:\n%s", out)
}
