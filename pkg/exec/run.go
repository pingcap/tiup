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
	"sync"
	"syscall"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/telemetry"
	"github.com/pingcap/tiup/pkg/tui/colorstr"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"golang.org/x/mod/semver"
)

// Skip displaying "Starting component ..." message for some commonly used components.
var skipStartingMessages = map[string]bool{
	"playground": true,
	"cluster":    true,
}

// RunComponent start a component and wait it
func RunComponent(env *environment.Environment, tag, spec, binPath string, args []string) error {
	component, version := environment.ParseCompVersion(spec)

	if version == "" {
		cmdCheckUpdate(component, version)
	}

	binPath, err := PrepareBinary(component, version, binPath)
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
		Version:     version,
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

	if skip, ok := skipStartingMessages[component]; !skip || !ok {
		colorstr.Fprintf(os.Stderr, "Starting component [bold]%s[reset]: %s\n", component, strings.Join(environment.HidePassword(c.Args), " "))
	}

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

	if err := utils.MkdirAll(p.InstanceDir, 0755); err != nil {
		return nil, err
	}

	sd := env.LocalPath(localdata.StorageParentDir, p.Component)
	if err := utils.MkdirAll(sd, 0755); err != nil {
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

func cmdCheckUpdate(component string, version utils.Version) {
	const (
		slowTimeout   = 1 * time.Second // Timeout to display checking message
		cancelTimeout = 2 * time.Second // Timeout to cancel the check
	)

	// This mutex is used for protecting flag as well as stdout
	mu := sync.Mutex{}
	isCheckFinished := false

	result := make(chan string, 1)

	go func() {
		time.Sleep(slowTimeout)
		mu.Lock()
		defer mu.Unlock()
		if !isCheckFinished {
			colorstr.Fprintf(os.Stderr, "Checking updates for component [bold]%s[reset]... ", component)
		}
	}()

	go func() {
		time.Sleep(cancelTimeout)
		result <- colorstr.Sprintf("[yellow]Timedout (after %s)", cancelTimeout)
	}()

	go func() {
		latestV, _, err := environment.GlobalEnv().V1Repository().LatestStableVersion(component, false)
		if err != nil {
			result <- ""
			return
		}
		selectVer, _ := environment.GlobalEnv().SelectInstalledVersion(component, version)

		if semver.Compare(selectVer.String(), latestV.String()) < 0 {
			result <- colorstr.Sprintf(`
[yellow]A new version of [bold]%[1]s[reset][yellow] is available:[reset] [red][bold]%[2]s[reset] -> [green][bold]%[3]s[reset]

    To update this component:   [tiup_command]tiup update %[1]s[reset]
    To update all components:   [tiup_command]tiup update --all[reset]
`,
				component, selectVer.String(), latestV.String())
		} else {
			result <- ""
		}
	}()

	s := <-result
	mu.Lock()
	defer mu.Unlock()
	isCheckFinished = true
	if len(s) > 0 {
		fmt.Fprintln(os.Stderr, s)
	}
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

func saveProcessInfo(p *PrepareCommandParams, c *exec.Cmd) {
	info := &localdata.Process{
		Component:   p.Component,
		CreatedTime: time.Now().Format(time.RFC3339),
		Pid:         c.Process.Pid,
		Exec:        c.Args[0],
		Args:        environment.HidePassword(c.Args),
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
