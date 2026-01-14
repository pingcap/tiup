package main

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	playgroundPIDFileName     = "pid"
	playgroundPortFileName    = "port"
	playgroundDaemonLogName   = "daemon.log"
	playgroundTUIEventLogName = "tuiv2.events.jsonl"
)

const pidFileWriteGracePeriod = 2 * time.Second

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if stdErrors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr interface{ Timeout() bool }
	return stdErrors.As(err, &netErr) && netErr.Timeout()
}

type pidFile struct {
	pid       int
	startedAt time.Time
	tag       string
}

func readPIDFile(path string) (pidFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return pidFile{}, err
	}

	var out pidFile
	seenPID := false

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "pid="):
			raw := strings.TrimSpace(strings.TrimPrefix(line, "pid="))
			if raw == "" {
				return pidFile{}, fmt.Errorf("pid is empty")
			}
			pid, err := strconv.Atoi(raw)
			if err != nil {
				return pidFile{}, fmt.Errorf("invalid pid %q: %w", raw, err)
			}
			out.pid = pid
			seenPID = true
		case strings.HasPrefix(line, "started_at="):
			raw := strings.TrimSpace(strings.TrimPrefix(line, "started_at="))
			if raw == "" {
				continue
			}
			startedAt, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return pidFile{}, fmt.Errorf("invalid started_at %q: %w", raw, err)
			}
			out.startedAt = startedAt
		case strings.HasPrefix(line, "tag="):
			out.tag = strings.TrimSpace(strings.TrimPrefix(line, "tag="))
		}
	}

	if !seenPID {
		return pidFile{}, fmt.Errorf("missing pid field")
	}

	return out, nil
}

func isPIDRunning(pid int) (running bool, err error) {
	if pid <= 0 {
		return false, fmt.Errorf("invalid pid %d", pid)
	}
	err = syscall.Kill(pid, 0)
	if err == nil || err == syscall.EPERM {
		return true, nil
	}
	if err == syscall.ESRCH {
		return false, nil
	}
	return false, err
}

func probePlaygroundCommandServer(ctx context.Context, port int) (bool, error) {
	if port <= 0 {
		return false, fmt.Errorf("invalid port %d", port)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	client := &http.Client{}

	pingReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/ping", port), nil)
	if err != nil {
		return false, errors.AddStack(err)
	}
	pingResp, err := client.Do(pingReq)
	if err != nil {
		if ctx.Err() != nil {
			return false, err
		}
	} else {
		defer pingResp.Body.Close()
		if pingResp.StatusCode == http.StatusOK {
			var reply CommandReply
			if err := json.NewDecoder(pingResp.Body).Decode(&reply); err != nil {
				return false, err
			}
			if reply.OK && strings.TrimSpace(reply.Message) == "pong" {
				return true, nil
			}
			return false, fmt.Errorf("unexpected ping response")
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/command", port), nil)
	if err != nil {
		return false, errors.AddStack(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var reply CommandReply
	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
		return false, err
	}

	if resp.StatusCode != http.StatusMethodNotAllowed {
		return false, fmt.Errorf("unexpected probe status: %s", resp.Status)
	}

	if !reply.OK && reply.Error == "method not allowed" {
		return true, nil
	}

	return false, fmt.Errorf("unexpected probe response")
}

func cleanupStaleRuntimeFiles(dataDir string) error {
	if strings.TrimSpace(dataDir) == "" {
		return fmt.Errorf("data dir is empty")
	}

	pidPath := filepath.Join(dataDir, playgroundPIDFileName)
	portPath := filepath.Join(dataDir, playgroundPortFileName)

	pid, err := readPIDFile(pidPath)
	switch {
	case err == nil:
		running, runErr := isPIDRunning(pid.pid)
		if runErr != nil {
			return errors.Annotatef(runErr, "check pid %d", pid.pid)
		}
		if running {
			return fmt.Errorf("playground already running (pid=%d)", pid.pid)
		}
		_ = os.Remove(pidPath)
		_ = os.Remove(portPath)
		return nil
	case !os.IsNotExist(err):
		info, statErr := os.Stat(pidPath)
		if statErr != nil && !os.IsNotExist(statErr) {
			return errors.AddStack(statErr)
		}
		if statErr == nil {
			age := time.Since(info.ModTime())
			if age >= 0 && age < pidFileWriteGracePeriod {
				return fmt.Errorf("playground is starting (pid file is being written)")
			}

			// The pid file is present but invalid (corrupted/partial). Use the port
			// probe as a safety net before treating it as stale.
			port, portErr := loadPort(dataDir)
			if portErr == nil && port > 0 {
				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				ok, probeErr := probePlaygroundCommandServer(ctx, port)
				cancel()
				if ok && probeErr == nil {
					return fmt.Errorf("playground already running (port=%d)", port)
				}
				if isTimeoutErr(probeErr) {
					return fmt.Errorf("playground command server probe timed out (port=%d)", port)
				}
			}

			_ = os.Remove(pidPath)
			_ = os.Remove(portPath)
			return nil
		}
	}

	port, err := loadPort(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.AddStack(err)
	}
	if port <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	ok, probeErr := probePlaygroundCommandServer(ctx, port)
	if ok && probeErr == nil {
		return fmt.Errorf("playground already running (port=%d)", port)
	}

	// A timeout means the server might still be running but is unresponsive.
	// Prefer failing the startup over risking concurrent writes to the same dir.
	if isTimeoutErr(probeErr) {
		return fmt.Errorf("playground command server probe timed out (port=%d)", port)
	}

	if stdErrors.Is(probeErr, syscall.ECONNREFUSED) {
		_ = os.Remove(portPath)
		return nil
	}
	_ = os.Remove(portPath)
	return nil
}

func claimPlaygroundPIDFile(dataDir, tag string) (release func(), err error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, fmt.Errorf("data dir is empty")
	}
	if strings.TrimSpace(tag) == "" {
		return nil, fmt.Errorf("tag is empty")
	}
	if err := utils.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	if err := cleanupStaleRuntimeFiles(dataDir); err != nil {
		return nil, errors.Annotatef(err, "tag %q is already in use", tag)
	}

	pidPath := filepath.Join(dataDir, playgroundPIDFileName)
	for {
		f, err := os.OpenFile(pidPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			now := time.Now().UTC().Format(time.RFC3339)
			_, writeErr := fmt.Fprintf(f, "pid=%d\nstarted_at=%s\ntag=%s\n", os.Getpid(), now, tag)
			closeErr := f.Close()
			if writeErr != nil {
				_ = os.Remove(pidPath)
				return nil, errors.AddStack(writeErr)
			}
			if closeErr != nil {
				_ = os.Remove(pidPath)
				return nil, errors.AddStack(closeErr)
			}
			return func() { _ = os.Remove(pidPath) }, nil
		}

		if !os.IsExist(err) {
			return nil, errors.AddStack(err)
		}
		if err := cleanupStaleRuntimeFiles(dataDir); err != nil {
			return nil, errors.Annotatef(err, "tag %q is already in use", tag)
		}
	}
}

func waitPlaygroundStopped(dataDir string, timeout time.Duration) error {
	if strings.TrimSpace(dataDir) == "" {
		return fmt.Errorf("data dir is empty")
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	deadline := time.Now().Add(timeout)
	pidPath := filepath.Join(dataDir, playgroundPIDFileName)
	portPath := filepath.Join(dataDir, playgroundPortFileName)

	for {
		pid, err := readPIDFile(pidPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			info, statErr := os.Stat(pidPath)
			if statErr != nil {
				if os.IsNotExist(statErr) {
					return nil
				}
			} else {
				age := time.Since(info.ModTime())
				if age >= pidFileWriteGracePeriod {
					port, portErr := loadPort(dataDir)
					stillRunning := false
					if portErr == nil && port > 0 {
						ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
						ok, probeErr := probePlaygroundCommandServer(ctx, port)
						cancel()
						stillRunning = (ok && probeErr == nil) || isTimeoutErr(probeErr)
					}
					if !stillRunning {
						_ = os.Remove(pidPath)
						_ = os.Remove(portPath)
						return nil
					}
				}
			}
		} else {
			running, runErr := isPIDRunning(pid.pid)
			if runErr == nil && !running {
				_ = os.Remove(pidPath)
				return nil
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for playground to stop")
		}
		time.Sleep(200 * time.Millisecond)
	}
}
