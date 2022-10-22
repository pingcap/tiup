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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/client"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/telemetry"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"golang.org/x/mod/semver"
)

// RunComponent start a component and wait it
func RunComponent(tiupC *client.Client, env *environment.Environment, tag, spec, binPath string, args []string) error {
	mirror, component, version, err := client.ParseComponentVersion(spec)
	if err != nil {
		return err
	}
	// component, version := environment.ParseCompVersion(spec)

	if version == "" {
		// cmdCheckUpdate(component, utils.Version(version), 2)
	}

	binPath, err = PrepareBinary2(tiupC, mirror, component, version, binPath)
	if err != nil {
		return err
	}

	// Clean data if current instance is a temporary
	clean := tag == "" && os.Getenv(localdata.EnvNameInstanceDataDir) == ""
	// p, err := launchComponent(component, version, binPath, tag, args, env)
	instanceDir := os.Getenv(localdata.EnvNameInstanceDataDir)
	if instanceDir == "" {
		// Generate a tag for current instance if the tag doesn't specified
		if tag == "" {
			tag = utils.Base62Tag()
		}
		instanceDir = env.LocalPath(localdata.DataParentDir, tag)
	}
	defer cleanDataDir(clean, instanceDir)

	params := &PrepareCommandParams{
		Component:   component,
		Version:     utils.Version(version),
		BinPath:     binPath,
		Tag:         tag,
		InstanceDir: instanceDir,
		Args:        args,
		Env:         env,
	}
	c, err := PrepareCommand(params)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Starting component `%s`: %s\n", component, strings.Join(c.Args, " "))
	err = c.Start()
	if err != nil {
		return errors.Annotatef(err, "Failed to start component `%s`", component)
	}
	// If the process has been launched, we must save the process info to meta directory
	saveProcessInfo(params, c)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range sc {
			sig := s.(syscall.Signal)
			fmt.Fprintf(os.Stderr, "Got signal %v (Component: %v ; PID: %v)\n", s, component, c.Process.Pid)
			if sig != syscall.SIGINT {
				_ = syscall.Kill(c.Process.Pid, sig)
			}
		}
	}()

	return c.Wait()
}

func cleanDataDir(rm bool, dir string) {
	if !rm {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		fmt.Fprintln(os.Stderr, "clean data directory failed: ", err.Error())
	}
}

// PrepareCommandParams for PrepareCommand.
type PrepareCommandParams struct {
	Component   string
	Version     utils.Version
	BinPath     string
	Tag         string
	InstanceDir string
	Args        []string
	Env         *environment.Environment
}

// PrepareCommand will download necessary component and returns a *exec.Cmd
func PrepareCommand(p *PrepareCommandParams) (*exec.Cmd, error) {
	env := p.Env
	profile := env.Profile()
	binPath := p.BinPath
	installPath := filepath.Dir(binPath)

	if err := os.MkdirAll(p.InstanceDir, 0755); err != nil {
		return nil, err
	}

	sd := env.LocalPath(localdata.StorageParentDir, p.Component)
	if err := os.MkdirAll(sd, 0755); err != nil {
		return nil, err
	}

	teleMeta, _, err := telemetry.GetMeta(env)
	if err != nil {
		return nil, err
	}

	tiupWd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	envs := []string{
		fmt.Sprintf("%s=%s", localdata.EnvNameHome, profile.Root()),
		fmt.Sprintf("%s=%s", localdata.EnvNameUserInputVersion, p.Version.String()),
		fmt.Sprintf("%s=%s", localdata.EnvNameTiUPVersion, version.NewTiUPVersion().SemVer()),
		fmt.Sprintf("%s=%s", localdata.EnvNameComponentDataDir, sd),
		fmt.Sprintf("%s=%s", localdata.EnvNameComponentInstallDir, installPath),
		fmt.Sprintf("%s=%s", localdata.EnvNameTelemetryStatus, teleMeta.Status),
		fmt.Sprintf("%s=%s", localdata.EnvNameTelemetryUUID, teleMeta.UUID),
		fmt.Sprintf("%s=%s", localdata.EnvNameTelemetrySecret, teleMeta.Secret),
		// to be removed in TiUP 2.0
		fmt.Sprintf("%s=%s", localdata.EnvNameWorkDir, tiupWd),
		fmt.Sprintf("%s=%s", localdata.EnvTag, p.Tag),
		fmt.Sprintf("%s=%s", localdata.EnvNameInstanceDataDir, p.InstanceDir),
	}
	envs = append(envs, os.Environ()...)

	// init the command
	c := exec.Command(binPath, p.Args...)

	c.Env = envs
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	return c, nil
}

func cmdCheckUpdate(component string, version utils.Version, timeoutSec int) {
	fmt.Fprintf(os.Stderr, "tiup is checking updates for component %s ...", component)
	updateC := make(chan string)
	// timeout for check update
	go func() {
		time.Sleep(time.Duration(timeoutSec) * time.Second)
		updateC <- color.RedString("timeout!")
	}()

	go func() {
		var updateInfo string
		latestV, _, err := environment.GlobalEnv().V1Repository().LatestStableVersion(component, false)
		if err != nil {
			return
		}
		selectVer, _ := environment.GlobalEnv().SelectInstalledVersion(component, version)

		if semver.Compare(selectVer.String(), latestV.String()) < 0 {
			updateInfo = fmt.Sprint(color.YellowString(`
A new version of %[1]s is available:
   The latest version:         %[2]s
   Local installed version:    %[3]s
   Update current component:   tiup update %[1]s
   Update all components:      tiup update --all
`,
				component, latestV.String(), selectVer.String()))
		}
		updateC <- updateInfo
	}()

	fmt.Fprintln(os.Stderr, <-updateC)
}

// PrepareBinary use given binpath or download from tiup mirror
func PrepareBinary(component string, version utils.Version, binPath string) (string, error) {
	if binPath != "" {
		tmp, err := filepath.Abs(binPath)
		if err != nil {
			return "", errors.Trace(err)
		}
		binPath = tmp
	} else {
		selectVer, err := environment.GlobalEnv().DownloadComponentIfMissing(component, version)
		if err != nil {
			return "", err
		}

		binPath, err = environment.GlobalEnv().BinaryPath(component, selectVer)
		if err != nil {
			return "", err
		}
	}
	return binPath, nil
}

// PrepareBinary2 use given binpath or download from tiup mirror
func PrepareBinary2(tiupC *client.Client, mirror, component, version, binPath string) (string, error) {
	if binPath != "" {
		tmp, err := filepath.Abs(binPath)
		if err != nil {
			return "", errors.Trace(err)
		}
		binPath = tmp
	} else {
		v1repo := tiupC.GetRepository(mirror)
		selectVer, err := v1repo.Local().GetComponentInstalledVersion(component, utils.Version(version))
		if err != nil {
			return "", err
		}
		binPath, err = v1repo.BinaryPath(v1repo.Local().ProfilePath(localdata.ComponentParentDir, mirror, component, selectVer.String()), component, selectVer.String())
		if err != nil {
			return "", err
		}
	}
	return binPath, nil
}

func saveProcessInfo(p *PrepareCommandParams, c *exec.Cmd) {
	info := &localdata.Process{
		Component:   p.Component,
		CreatedTime: time.Now().Format(time.RFC3339),
		Pid:         c.Process.Pid,
		Exec:        c.Args[0],
		Args:        c.Args,
		Dir:         p.InstanceDir,
		Env:         c.Env,
		Cmd:         c,
	}
	metaFile := filepath.Join(info.Dir, localdata.MetaFilename)
	file, err := os.OpenFile(metaFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err == nil {
		defer file.Close()
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "    ")
		_ = encoder.Encode(info)
	}
}
