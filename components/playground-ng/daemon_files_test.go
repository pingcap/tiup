package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaimPlaygroundPIDFile_CreatesAndReleases(t *testing.T) {
	base := t.TempDir()

	release, err := claimPlaygroundPIDFile(base, "test")
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(base, playgroundPIDFileName))

	release()
	_, err = os.Stat(filepath.Join(base, playgroundPIDFileName))
	require.True(t, os.IsNotExist(err))
}

func TestClaimPlaygroundPIDFile_RunningPIDRejects(t *testing.T) {
	base := t.TempDir()
	pidPath := filepath.Join(base, playgroundPIDFileName)
	require.NoError(t, os.WriteFile(pidPath, []byte("pid="+strconv.Itoa(os.Getpid())+"\n"), 0o644))

	_, err := claimPlaygroundPIDFile(base, "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "already in use")
}

func TestClaimPlaygroundPIDFile_PortOnlyRunningRejects(t *testing.T) {
	base := t.TempDir()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/command" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "method not allowed"})
	}))
	defer s.Close()

	u, err := url.Parse(s.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	require.NoError(t, dumpPort(filepath.Join(base, playgroundPortFileName), port))

	_, err = claimPlaygroundPIDFile(base, "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "already in use")
	_, err = os.Stat(filepath.Join(base, playgroundPIDFileName))
	require.True(t, os.IsNotExist(err))
}

func TestCleanupStaleRuntimeFiles_RemovesStalePIDAndPort(t *testing.T) {
	base := t.TempDir()

	stalePID := 999999
	found := false
	for i := 0; i < 1000; i++ {
		pid := stalePID + i
		running, err := isPIDRunning(pid)
		if err == nil && !running {
			stalePID = pid
			found = true
			break
		}
	}
	require.True(t, found, "cannot find a stale pid")

	require.NoError(t, os.WriteFile(filepath.Join(base, playgroundPIDFileName), []byte("pid="+strconv.Itoa(stalePID)+"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(base, playgroundPortFileName), []byte("12345"), 0o644))

	require.NoError(t, cleanupStaleRuntimeFiles(base))
	_, err := os.Stat(filepath.Join(base, playgroundPIDFileName))
	require.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(base, playgroundPortFileName))
	require.True(t, os.IsNotExist(err))
}

func TestCleanupStaleRuntimeFiles_RemovesStalePort(t *testing.T) {
	base := t.TempDir()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())

	require.NoError(t, os.WriteFile(filepath.Join(base, playgroundPortFileName), []byte(strconv.Itoa(port)), 0o644))

	require.NoError(t, cleanupStaleRuntimeFiles(base))
	_, err = os.Stat(filepath.Join(base, playgroundPortFileName))
	require.True(t, os.IsNotExist(err))
}
