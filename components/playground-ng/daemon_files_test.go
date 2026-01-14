package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReadPIDFile_ParsesStartedAtAndTag(t *testing.T) {
	base := t.TempDir()
	pidPath := filepath.Join(base, playgroundPIDFileName)

	startedAt := "2026-01-13T20:00:00Z"
	require.NoError(t, os.WriteFile(pidPath, []byte(fmt.Sprintf("pid=123\nstarted_at=%s\ntag=my-cluster\n", startedAt)), 0o644))

	got, err := readPIDFile(pidPath)
	require.NoError(t, err)
	require.Equal(t, 123, got.pid)
	require.Equal(t, "my-cluster", got.tag)
	require.Equal(t, time.Date(2026, 1, 13, 20, 0, 0, 0, time.UTC), got.startedAt)
}

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

func TestCleanupStaleRuntimeFiles_PortProbeTimeoutTreatedAsInUse(t *testing.T) {
	base := t.TempDir()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second)
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

	err = cleanupStaleRuntimeFiles(base)
	require.Error(t, err)
	require.Contains(t, err.Error(), "timed out")
	require.FileExists(t, filepath.Join(base, playgroundPortFileName))
}

func TestCleanupStaleRuntimeFiles_InvalidPIDTimeoutTreatedAsInUse(t *testing.T) {
	base := t.TempDir()

	pidPath := filepath.Join(base, playgroundPIDFileName)
	require.NoError(t, os.WriteFile(pidPath, []byte(""), 0o644))
	old := time.Now().Add(-time.Minute)
	require.NoError(t, os.Chtimes(pidPath, old, old))

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second)
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

	err = cleanupStaleRuntimeFiles(base)
	require.Error(t, err)
	require.Contains(t, err.Error(), "timed out")
	require.FileExists(t, pidPath)
	require.FileExists(t, filepath.Join(base, playgroundPortFileName))
}

func TestCleanupStaleRuntimeFiles_StaleInvalidPIDIsRemoved(t *testing.T) {
	base := t.TempDir()

	pidPath := filepath.Join(base, playgroundPIDFileName)
	require.NoError(t, os.WriteFile(pidPath, []byte(""), 0o644))
	portPath := filepath.Join(base, playgroundPortFileName)
	require.NoError(t, os.WriteFile(portPath, []byte("12345"), 0o644))

	old := time.Now().Add(-time.Minute)
	require.NoError(t, os.Chtimes(pidPath, old, old))

	require.NoError(t, cleanupStaleRuntimeFiles(base))
	_, err := os.Stat(pidPath)
	require.True(t, os.IsNotExist(err))
	_, err = os.Stat(portPath)
	require.True(t, os.IsNotExist(err))
}

func TestCleanupStaleRuntimeFiles_RecentInvalidPIDIsTreatedAsInUse(t *testing.T) {
	base := t.TempDir()

	pidPath := filepath.Join(base, playgroundPIDFileName)
	require.NoError(t, os.WriteFile(pidPath, []byte(""), 0o644))
	now := time.Now()
	require.NoError(t, os.Chtimes(pidPath, now, now))

	err := cleanupStaleRuntimeFiles(base)
	require.Error(t, err)
	require.FileExists(t, pidPath)
}

func TestProbePlaygroundCommandServer_PingSucceeds(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ping":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(CommandReply{OK: true, Message: "pong"})
		case "/command":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "probe should not call /command when ping works"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	u, err := url.Parse(s.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ok, err := probePlaygroundCommandServer(ctx, port)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestProbePlaygroundCommandServer_FallbackToCommandWhenPingMissing(t *testing.T) {
	commandCalled := false
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ping":
			w.WriteHeader(http.StatusNotFound)
		case "/command":
			commandCalled = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "method not allowed"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	u, err := url.Parse(s.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ok, err := probePlaygroundCommandServer(ctx, port)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, commandCalled)
}

func TestWaitPlaygroundStopped_StaleInvalidPIDIsRemoved(t *testing.T) {
	base := t.TempDir()

	pidPath := filepath.Join(base, playgroundPIDFileName)
	require.NoError(t, os.WriteFile(pidPath, []byte(""), 0o644))

	old := time.Now().Add(-time.Minute)
	require.NoError(t, os.Chtimes(pidPath, old, old))

	require.NoError(t, waitPlaygroundStopped(base, time.Second))
	_, err := os.Stat(pidPath)
	require.True(t, os.IsNotExist(err))
}
