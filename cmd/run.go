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

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pingcap-incubator/tiup/pkg/repository/v0manifest"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap/errors"
)

func runComponent(env *meta.Environment, tag, spec, binPath string, args []string) error {
	component, version := meta.ParseCompVersion(spec)
	if !isSupportedComponent(env, component) {
		return fmt.Errorf("component `%s` does not support `%s/%s` (see `tiup list --refresh`)", component, runtime.GOOS, runtime.GOARCH)
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
		ch <- p.cmd.Wait()
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

func isSupportedComponent(env *meta.Environment, component string) bool {
	// check local manifest
	manifest := env.Profile().Manifest()
	if manifest != nil && manifest.HasComponent(component) {
		return true
	}

	manifest, err := env.Repository().Manifest()
	if err != nil {
		fmt.Println("Fetch latest manifest error:", err)
		return false
	}
	if err := env.Profile().SaveManifest(manifest); err != nil {
		fmt.Println("Save latest manifest error:", err)
	}
	comp, found := manifest.FindComponent(component)
	if !found {
		return false
	}
	return comp.IsSupport(runtime.GOOS, runtime.GOARCH)
}

type process struct {
	Component   string   `json:"component"`
	CreatedTime string   `json:"created_time"`
	Pid         int      `json:"pid"`            // PID of the process
	Exec        string   `json:"exec"`           // Path to the binary
	Args        []string `json:"args,omitempty"` // Command line arguments
	Env         []string `json:"env,omitempty"`  // Environment variables
	Dir         string   `json:"dir,omitempty"`  // Working directory
	cmd         *exec.Cmd
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

func launchComponent(ctx context.Context, component string, version v0manifest.Version, binPath string, tag string, args []string, env *meta.Environment) (*process, error) {
	selectVer, err := env.DownloadComponentIfMissing(component, version)
	if err != nil {
		return nil, err
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
		binPath, err = profile.BinaryPath(component, selectVer)
		if err != nil {
			return nil, err
		}
	}

	wd := os.Getenv(localdata.EnvNameInstanceDataDir)
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

	teleMeta, _, err := getTelemetryMeta(env)
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

	p := &process{
		Component:   component,
		CreatedTime: time.Now().Format(time.RFC3339),
		Exec:        binPath,
		Args:        args,
		Dir:         wd,
		Env:         envs,
		cmd:         c,
	}

	fmt.Printf("Starting component `%s`: %s\n", component, strings.Join(append([]string{p.Exec}, p.Args...), " "))
	err = p.cmd.Start()
	if p.cmd.Process != nil {
		p.Pid = p.cmd.Process.Pid
	}
	return p, err
}
