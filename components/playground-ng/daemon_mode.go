package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/tui/colorstr"
	tuiterm "github.com/pingcap/tiup/pkg/tui/term"
	tuiv2output "github.com/pingcap/tiup/pkg/tuiv2/output"
)

func runBackgroundStarter(state *cliState) error {
	if state == nil {
		return fmt.Errorf("cli state is nil")
	}
	if state.dryRun {
		return fmt.Errorf("--background is not supported with --dry-run")
	}
	if strings.TrimSpace(state.tag) == "" {
		return fmt.Errorf("tag is empty")
	}
	if strings.TrimSpace(state.dataDir) == "" {
		return fmt.Errorf("data dir is empty")
	}

	if err := cleanupStaleRuntimeFiles(state.dataDir); err != nil {
		return errors.Annotatef(err, "tag %q is already in use", state.tag)
	}

	if err := os.MkdirAll(state.dataDir, 0o755); err != nil {
		return errors.AddStack(err)
	}

	logPath := filepath.Join(state.dataDir, playgroundDaemonLogName)
	offset := int64(0)
	if st, err := os.Stat(logPath); err == nil {
		offset = st.Size()
	}

	logWriter, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return errors.AddStack(err)
	}

	args := buildDaemonArgs(state.tag)
	exe, err := os.Executable()
	if err != nil {
		_ = logWriter.Close()
		return errors.AddStack(err)
	}

	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter

	devNull, err := os.OpenFile("/dev/null", os.O_RDONLY, 0)
	if err != nil {
		_ = logWriter.Close()
		return errors.AddStack(err)
	}
	defer devNull.Close()
	cmd.Stdin = devNull

	cmd.Env = daemonEnv()

	tailCtx, cancelTail := context.WithCancel(context.Background())
	defer cancelTail()
	go tailFile(tailCtx, logPath, offset, tuiv2output.Stdout.Get())

	waitCh := make(chan error, 1)

	if err := cmd.Start(); err != nil {
		_ = logWriter.Close()
		return errors.Annotate(err, "start playground daemon")
	}
	_ = logWriter.Close()

	go func() {
		waitCh <- cmd.Wait()
	}()

	// Best-effort: if users interrupt the starter, terminate the daemon we just
	// started to avoid leaving an unexpected background cluster.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(sigCh)

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case sig := <-sigCh:
			_ = cmd.Process.Signal(sig)
			return fmt.Errorf("starter interrupted by signal %v", sig)
		case err := <-waitCh:
			if err == nil {
				return fmt.Errorf("playground daemon exited before ready")
			}
			return errors.Annotate(err, "playground daemon exited before ready")
		case <-ticker.C:
			port, err := loadPort(state.dataDir)
			if err != nil || port <= 0 {
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			ok, probeErr := probePlaygroundCommandServer(ctx, port)
			cancel()
			if ok && probeErr == nil {
				cancelTail()
				out := tuiv2output.Stdout.Get()
				colorstr.Fprintf(out, backgroundStarterReadyMessage(state.tag))
				return nil
			}
		}
	}
}

func backgroundStarterReadyMessage(tag string) string {
	return fmt.Sprintf("\n[dim]Cluster running in background (-d).[reset]\n[dim]To stop: [reset]tiup playground-ng stop --tag %s\n", tag)
}

func daemonEnv() []string {
	env := append([]string(nil), os.Environ()...)

	// If the starter is running in a color-capable terminal, enable ANSI colors
	// for the daemon output even though it is redirected to a file.
	if os.Getenv(tuiterm.EnvNoColor) != "" {
		return env
	}
	if os.Getenv(tuiterm.EnvForceTTY) != "" {
		return env
	}
	if os.Getenv(tuiterm.EnvForceColor) != "" {
		return env
	}
	if tuiterm.ResolveFile(os.Stderr).Color {
		env = append(env, tuiterm.EnvForceColor+"=1")
	}
	return env
}

func buildDaemonArgs(tag string) []string {
	rawArgs := os.Args[1:]
	out := make([]string, 0, len(rawArgs)+3)
	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		switch {
		case arg == "--background" || arg == "-d" || strings.HasPrefix(arg, "--background="):
			continue
		case arg == "--run-as-daemon" || strings.HasPrefix(arg, "--run-as-daemon="):
			continue
		case arg == "--tag" || arg == "-T":
			if i+1 < len(rawArgs) {
				i++
			}
			continue
		case strings.HasPrefix(arg, "--tag="):
			continue
		case strings.HasPrefix(arg, "-T") && len(arg) > 2:
			continue
		default:
			out = append(out, arg)
		}
	}
	out = append(out, "--tag", tag, "--run-as-daemon")
	return out
}

func tailFile(ctx context.Context, path string, offset int64, out io.Writer) {
	if out == nil {
		out = io.Discard
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	if offset > 0 {
		_, _ = f.Seek(offset, io.SeekStart)
	}

	buf := make([]byte, 32*1024)
	for {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
		}

		n, err := f.Read(buf)
		if n > 0 {
			_, _ = out.Write(buf[:n])
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		return
	}
}
