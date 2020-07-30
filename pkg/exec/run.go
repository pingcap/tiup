// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"github.com/pingcap/tiup/pkg/telemetry"
	"golang.org/x/mod/semver"
)

// RunComponent start a component and wait it
func RunComponent(env *environment.Environment, tag, spec, binPath string, args []string) error {
	component, version := environment.ParseCompVersion(spec)
	if !env.IsSupportedComponent(component) {
		return fmt.Errorf("component `%s` does not support `%s/%s` (see `tiup list`)", component, runtime.GOOS, runtime.GOARCH)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Clean data if current instance is a temporary
	clean := tag == "" && os.Getenv(localdata.EnvNameInstanceDataDir) == ""

	p, err := launchComponent(ctx, component, version, binPath, tag, args, env)
	// If the process has been launched, we must save the process info to meta directory
	if err == nil || (p != nil && p.Pid != 0) {
		defer cleanDataDir(clean, p.Dir)
		metaFile := filepath.Join(p.Dir, localdata.MetaFilename)
		file, err := os.OpenFile(metaFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
		if err == nil {
			defer file.Close()
			encoder := json.NewEncoder(file)
			encoder.SetIndent("", "    ")
			_ = encoder.Encode(p)
		}
	}
	if err != nil {
		fmt.Printf("Failed to start component `%s`\n", component)
		return err
	}

	ch := make(chan error)
	var sig syscall.Signal
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	defer func() {
		for err := range ch {
			if err != nil {
				errs := err.Error()
				if strings.Contains(errs, "signal") ||
					(sig == syscall.SIGINT && strings.Contains(errs, "exit status 1")) {
					continue
				}
				fmt.Printf("Component `%s` exit with error: %s\n", component, errs)
				return
			}
		}
	}()

	go func() {
		defer close(ch)
		ch <- p.Cmd.Wait()
	}()

	select {
	case s := <-sc:
		sig = s.(syscall.Signal)
		fmt.Printf("Got signal %v (Component: %v ; PID: %v)\n", s, component, p.Pid)
		if component == "tidb" {
			return syscall.Kill(p.Pid, syscall.SIGKILL)
		} else if sig != syscall.SIGINT {
			return syscall.Kill(p.Pid, sig)
		} else {
			return nil
		}

	case err := <-ch:
		return errors.Annotatef(err, "run `%s` (wd:%s) failed", p.Exec, p.Dir)
	}
}

func cleanDataDir(rm bool, dir string) {
	if !rm {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		fmt.Println("clean data directory failed: ", err.Error())
	}
}

func base62Tag() string {
	const base = 62
	const sets = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 0)
	num := time.Now().UnixNano() / int64(time.Millisecond)
	for num > 0 {
		r := math.Mod(float64(num), float64(base))
		num /= base
		b = append([]byte{sets[int(r)]}, b...)
	}
	return string(b)
}

// PrepareCommand will download necessary component and returns a *exec.Cmd
func PrepareCommand(
	ctx context.Context,
	component string,
	version v0manifest.Version,
	binPath, tag, wd string,
	args []string,
	env *environment.Environment,
	checkUpdate ...bool,
) (*exec.Cmd, error) {
	selectVer, err := env.DownloadComponentIfMissing(component, version)
	if err != nil {
		return nil, err
	}

	if version.IsEmpty() && len(checkUpdate) > 0 && checkUpdate[0] {
		latestV, _, err := env.V1Repository().LatestStableVersion(component, true)
		if err != nil {
			return nil, err
		}
		if semver.Compare(selectVer.String(), latestV.String()) < 0 {
			fmt.Println(color.YellowString(`Found %[1]s newer version:

    The latest version:         %[2]s
    Local installed version:    %[3]s
    Update current component:   tiup update %[1]s
    Update all components:      tiup update --all
`,
				component, latestV.String(), selectVer.String()))
		}
	}

	// playground && cluster version must greater than v1.0.0
	if (component == "playground" || component == "cluster") && semver.Compare(selectVer.String(), "v1.0.0") < 0 {
		return nil, errors.Errorf("incompatible component version, please use `tiup update %s` to upgrade to the latest version", component)
	}

	profile := env.Profile()
	installPath, err := profile.ComponentInstalledPath(component, selectVer)
	if err != nil {
		return nil, err
	}

	if binPath != "" {
		p, err := filepath.Abs(binPath)
		if err != nil {
			return nil, errors.Trace(err)
		}
		binPath = p
	} else {
		binPath, err = env.BinaryPath(component, selectVer)
		if err != nil {
			return nil, err
		}
	}

	if wd == "" {
		// Generate a tag for current instance if the tag doesn't specified
		if tag == "" {
			tag = base62Tag()
		}
		wd = env.LocalPath(localdata.DataParentDir, tag)
	}
	if err := os.MkdirAll(wd, 0755); err != nil {
		return nil, err
	}

	sd := env.LocalPath(localdata.StorageParentDir, component)
	if err := os.MkdirAll(sd, 0755); err != nil {
		return nil, err
	}

	tiupWd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	teleMeta, _, err := telemetry.GetMeta(env)
	if err != nil {
		return nil, err
	}

	envs := []string{
		fmt.Sprintf("%s=%s", localdata.EnvNameHome, profile.Root()),
		fmt.Sprintf("%s=%s", localdata.EnvNameWorkDir, tiupWd),
		fmt.Sprintf("%s=%s", localdata.EnvNameInstanceDataDir, wd),
		fmt.Sprintf("%s=%s", localdata.EnvNameComponentDataDir, sd),
		fmt.Sprintf("%s=%s", localdata.EnvNameComponentInstallDir, installPath),
		fmt.Sprintf("%s=%s", localdata.EnvNameTelemetryStatus, teleMeta.Status),
		fmt.Sprintf("%s=%s", localdata.EnvNameTelemetryUUID, teleMeta.UUID),
		fmt.Sprintf("%s=%s", localdata.EnvTag, tag),
	}

	// init the command
	c := exec.CommandContext(ctx, binPath, args...)
	c.Env = append(
		envs,
		os.Environ()...,
	)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Dir = wd

	return c, nil
}

func launchComponent(ctx context.Context, component string, version v0manifest.Version, binPath string, tag string, args []string, env *environment.Environment) (*localdata.Process, error) {
	wd := os.Getenv(localdata.EnvNameInstanceDataDir)
	c, err := PrepareCommand(ctx, component, version, binPath, tag, wd, args, env, true)
	if err != nil {
		return nil, err
	}

	p := &localdata.Process{
		Component:   component,
		CreatedTime: time.Now().Format(time.RFC3339),
		Exec:        c.Args[0],
		Args:        args,
		Dir:         c.Dir,
		Env:         c.Env,
		Cmd:         c,
	}

	fmt.Printf("Starting component `%s`: %s\n", component, strings.Join(append([]string{p.Exec}, p.Args...), " "))
	err = p.Cmd.Start()
	if p.Cmd.Process != nil {
		p.Pid = p.Cmd.Process.Pid
	}
	return p, err
}
