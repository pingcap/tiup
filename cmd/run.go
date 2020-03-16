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
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap/errors"
	"golang.org/x/mod/semver"
)

func runComponent(tag, spec string, args []string, rm bool) error {
	component, version := meta.ParseCompVersion(spec)
	if !isSupportedComponent(component) {
		return fmt.Errorf("unkonwn component `%s` (see supported components via `tiup list --refresh`)", component)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p, err := launchComponent(ctx, component, version, tag, args)
	// If the process has been launched, we must save the process info to meta directory
	if err == nil || (p != nil && p.Pid != 0) {
		defer cleanDataDir(rm, p.Dir)
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
	go func() {
		defer close(ch)

		fmt.Printf("Starting %s %s\n", p.Exec, strings.Join(p.Args, " "))
		ch <- p.cmd.Wait()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case s := <-sig:
		fmt.Printf("Got signal %v (Component: %v ; PID: %v)\n", s, component, p.Pid)
		if component == "tidb" {
			return syscall.Kill(p.Pid, syscall.SIGKILL)
		}
		return syscall.Kill(p.Pid, s.(syscall.Signal))

	case err := <-ch:
		return errors.Annotatef(err, "start `%s` (wd:%s) failed", p.Exec, p.Dir)
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

func isSupportedComponent(component string) bool {
	// check local manifest
	manifest := profile.Manifest()
	if manifest != nil && manifest.HasComponent(component) {
		return true
	}

	manifest, err := repository.Manifest()
	if err != nil {
		fmt.Println("Fetch latest manifest error:", err)
		return false
	}
	if err := profile.SaveManifest(manifest); err != nil {
		fmt.Println("Save latest manifest error:", err)
	}
	return manifest.HasComponent(component)
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

func launchComponent(ctx context.Context, component string, version meta.Version, tag string, args []string) (*process, error) {
	binPath, err := downloadIfMissing(component, version)
	if err != nil {
		return nil, err
	}
	installPath, err := profile.ComponentInstallPath(component, version)
	if err != nil {
		return nil, err
	}

	wd := os.Getenv(localdata.EnvNameInstanceDataDir)
	if wd == "" {
		// Generate a tag for current instance if the tag doesn't specified
		if tag == "" {
			tag = base62Tag()
		}
		wd = profile.Path(filepath.Join(localdata.DataParentDir, tag))
	}
	if err := os.MkdirAll(wd, 0755); err != nil {
		return nil, err
	}

	tiupWd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	envs := []string{
		fmt.Sprintf("%s=%s", localdata.EnvNameHome, profile.Root()),
		fmt.Sprintf("%s=%s", localdata.EnvNameWorkDir, tiupWd),
		fmt.Sprintf("%s=%s", localdata.EnvNameInstanceDataDir, wd),
		fmt.Sprintf("%s=%s", localdata.EnvNameComponentInstallDir, installPath),
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

	err = p.cmd.Start()
	if p.cmd.Process != nil {
		p.Pid = p.cmd.Process.Pid
	}
	return p, err
}

func downloadIfMissing(component string, version meta.Version) (string, error) {
	versions, err := profile.InstalledVersions(component)
	if err != nil {
		return "", err
	}

	// Use the latest version if user doesn't specify a specific version and
	// download the latest version if the specific component doesn't be installed

	// Check whether the specific version exist in local
	if version.IsEmpty() && len(versions) > 0 {
		sort.Slice(versions, func(i, j int) bool {
			return semver.Compare(versions[i], versions[j]) < 0
		})
		version = meta.Version(versions[len(versions)-1])
	}

	needDownload := version.IsEmpty()
	if !version.IsEmpty() {
		installed := false
		for _, v := range versions {
			if meta.Version(v) == version {
				installed = true
				break
			}
		}
		needDownload = !installed
	}

	if needDownload {
		fmt.Printf("The component `%s` doesn't installed, download from repository\n", component)
		manifest, err := repository.ComponentVersions(component)
		if err != nil {
			return "", errors.Trace(err)
		}
		err = profile.SaveVersions(component, manifest)
		if err != nil {
			return "", errors.Trace(err)
		}
		if version.IsEmpty() {
			version = manifest.LatestVersion()
		}
		compDir := profile.ComponentsDir()
		spec := fmt.Sprintf("%s:%s", component, version)
		err = repository.DownloadComponent(compDir, spec)
		if err != nil {
			return "", errors.Trace(err)
		}
	}

	return profile.BinaryPath(component, version)
}
