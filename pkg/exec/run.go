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
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/telemetry"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"golang.org/x/mod/semver"
)

// RunComponent start a component and wait it
func RunComponent(env *environment.Environment, tag, spec, binPath string, args []string) error {
	component, version := environment.ParseCompVersion(spec)

	if version == "" {
		fmt.Fprintf(os.Stderr, "tiup is checking updates for component %s ...", component)
		updateC := make(chan string)
		// timeout for check update
		go func() {
			time.Sleep(2 * time.Second)
			updateC <- color.RedString("timeout!")
		}()

		go func() {
			var updateInfo string
			if version.IsEmpty() {
				latestV, _, err := env.V1Repository().LatestStableVersion(component, false)
				if err != nil {
					return
				}
				selectVer, _ := env.SelectInstalledVersion(component, version)

				if semver.Compare(selectVer.String(), latestV.String()) < 0 {
					updateInfo = fmt.Sprint(color.YellowString(`
Found %[1]s newer version:
   	The latest version:         %[2]s
   	Local installed version:    %[3]s
   	Update current component:   tiup update %[1]s
   	Update all components:      tiup update --all
`,
						component, latestV.String(), selectVer.String()))
				}
			}
			updateC <- updateInfo
		}()

		fmt.Fprintln(os.Stderr, <-updateC)
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
		fmt.Fprintf(os.Stderr, "Failed to start component `%s`\n", component)
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
				fmt.Fprintf(os.Stderr, "Component `%s` exit with error: %s\n", component, errs)
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
		fmt.Fprintf(os.Stderr, "Got signal %v (Component: %v ; PID: %v)\n", s, component, p.Pid)
		if component == "tidb" {
			return syscall.Kill(p.Pid, syscall.SIGKILL)
		}
		if sig != syscall.SIGINT {
			return syscall.Kill(p.Pid, sig)
		}
		return nil

	case err := <-ch:
		return errors.Annotatef(err, "run `%s` (wd:%s) failed", p.Exec, p.Dir)
	}
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
	Ctx          context.Context
	Component    string
	Version      utils.Version
	BinPath      string
	Tag          string
	InstanceDir  string
	WD           string
	Args         []string
	EnvVariables []string
	SysProcAttr  *syscall.SysProcAttr
	Env          *environment.Environment
	CheckUpdate  bool
}

// PrepareCommand will download necessary component and returns a *exec.Cmd
func PrepareCommand(p *PrepareCommandParams) (*exec.Cmd, error) {
	env := p.Env
	profile := env.Profile()
	binPath := p.BinPath

	if binPath != "" {
		tmp, err := filepath.Abs(binPath)
		if err != nil {
			return nil, errors.Trace(err)
		}
		binPath = tmp
	} else {
		selectVer, err := env.DownloadComponentIfMissing(p.Component, p.Version)
		if err != nil {
			return nil, err
		}

		// playground && cluster version must greater than v1.0.0
		if (p.Component == "playground" || p.Component == "cluster") && semver.Compare(selectVer.String(), "v1.0.0") < 0 {
			return nil, errors.Errorf("incompatible component version, please use `tiup update %s` to upgrade to the latest version", p.Component)
		}

		binPath, err = env.BinaryPath(p.Component, selectVer)
		if err != nil {
			return nil, err
		}
	}
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
	envs = append(envs, p.EnvVariables...)

	// init the command
	c := exec.CommandContext(p.Ctx, binPath, p.Args...)
	c.Env = envs
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Dir = p.WD
	c.SysProcAttr = p.SysProcAttr

	return c, nil
}

func launchComponent(ctx context.Context, component string, version utils.Version, binPath string, tag string, args []string, env *environment.Environment) (*localdata.Process, error) {
	instanceDir := os.Getenv(localdata.EnvNameInstanceDataDir)
	if instanceDir == "" {
		// Generate a tag for current instance if the tag doesn't specified
		if tag == "" {
			tag = utils.Base62Tag()
		}
		instanceDir = env.LocalPath(localdata.DataParentDir, tag)
	}

	if len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
		}
	}

	params := &PrepareCommandParams{
		Ctx:         ctx,
		Component:   component,
		Version:     version,
		BinPath:     binPath,
		Tag:         tag,
		InstanceDir: instanceDir,
		WD:          "",
		Args:        args,
		SysProcAttr: nil,
		Env:         env,
		CheckUpdate: true,
	}
	c, err := PrepareCommand(params)
	if err != nil {
		return nil, err
	}

	p := &localdata.Process{
		Component:   component,
		CreatedTime: time.Now().Format(time.RFC3339),
		Exec:        c.Args[0],
		Args:        args,
		Dir:         instanceDir,
		Env:         c.Env,
		Cmd:         c,
	}

	fmt.Fprintf(os.Stderr, "Starting component `%s`: %s\n", component, strings.Join(append([]string{p.Exec}, p.Args...), " "))
	err = p.Cmd.Start()
	if p.Cmd.Process != nil {
		p.Pid = p.Cmd.Process.Pid
	}
	return p, err
}
