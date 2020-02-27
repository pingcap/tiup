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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/c4pt0r/tiup/pkg/meta"
	"github.com/c4pt0r/tiup/pkg/profile"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const (
	compTypeMeta    = "pd"
	compTypeStorage = "tikv"
	compTypeCompute = "tidb"
)

func newRunCmd() *cobra.Command {
	var (
		version   string
		component string
	)

	cmdLaunch := &cobra.Command{
		Use:   "run <component1>:[version]",
		Short: "Run a component of specific version",
		Long: `Launch a TiDB component process of specific version.
There are 3 types of component in "tidb-core":
  meta:     Metadata nodes of the cluster, the PD server
  storage:  Storage nodes, the TiKV server
  compute:  SQL layer and compute nodes, the TiDB server`,
		Example: "tiup launch meta v3.0.8",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			ss := strings.Split(args[0], ":")
			component = ss[0]
			if len(ss) > 1 {
				version = ss[1]
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Launching process of %s %s\n", component, version)
			p, err := launchComponentProcess(version, component, args[1:])
			if err != nil {
				if p != nil && p.Pid != 0 {
					fmt.Printf("Error occured, but the process may be already started with PID %d\n", p.Pid)
				}
				return err
			}
			fmt.Printf("Started %s %s...\n", p.Exec, strings.Join(p.Args, " "))
			fmt.Printf("Process %d started for %s %s\n", p.Pid, component, version)
			return nil
		},
	}

	return cmdLaunch
}

func launchComponentProcess(ver, comp string, args []string) (*compProcess, error) {
	binPath, err := getServerBinPath(ver, comp)
	if err != nil {
		return nil, err
	}

	profileDir := profile.MustDir()
	p := &compProcess{
		Exec: binPath,
		Args: args,
		Dir:  path.Join(profileDir, "data", comp),
		Env:  []string{"TIUP_HOME=" + profileDir},
	}

	//fmt.Printf("%s %s\n", binPath, args)
	if err := p.Launch(false); err != nil {
		return p, err
	}

	return p, saveProcessToList(p)
}

func getServerBinPath(ver, comp string) (string, error) {
	if ver != "" {
		return getBinPath(comp, ver)
	}

	files, err := ioutil.ReadDir(path.Join(profile.MustDir(), "components", comp))
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("can't find binary for %s", comp)
	}

	// If this component is not installed, we should install the last version of it
	if len(files) == 0 {
		if ver, err := downloadNewestComponent(comp); err != nil {
			return "", errors.Trace(err)
		} else {
			return getBinPath(comp, string(ver))
		}
	}

	// Choose the latest
	for _, file := range files {
		if semver.Compare(file.Name(), ver) > 0 {
			ver = file.Name()
		}
	}
	return getBinPath(comp, ver)
}

func downloadNewestComponent(comp string) (meta.Version, error) {
	mirror := meta.NewMirror(defaultMirror)
	if err := mirror.Open(); err != nil {
		return "", errors.Trace(err)
	}
	defer mirror.Close()

	repo := meta.NewRepository(mirror)
	v, err := repo.ComponentVersions(comp)
	if err != nil {
		return "", errors.Trace(err)
	}

	lastVer := v.Versions[len(v.Versions)-1]
	if err := repo.Download(comp, lastVer.Version); err != nil {
		return "", errors.Trace(err)
	}

	return lastVer.Version, nil
}
