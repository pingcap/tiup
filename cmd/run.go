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
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/c4pt0r/tiup/pkg/meta"
	"github.com/c4pt0r/tiup/pkg/tui"
	"github.com/c4pt0r/tiup/pkg/utils"

	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const (
	processListFilename = "processes.json"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <component1>:[version]",
		Short: "Run a component of specific version",
		Long: `Launch a TiDB component process of specific version.
There are 3 types of component in "tidb-core":
  meta:     Metadata nodes of the cluster, the PD server
  storage:  Storage nodes, the TiKV server
  compute:  SQL layer and compute nodes, the TiDB server`,
		Example:            "tiup run playground",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fs := flag.NewFlagSet("run", flag.ContinueOnError)
			name := fs.String("name", "default", "--name=<name>")
			if err := fs.Parse(args); err != nil {
				return err
			}
			args = fs.Args()
			if len(args) == 0 {
				return cmd.Help()
			}

			component := args[0]
			fmt.Printf("Launching process of %s\n", component)
			p, err := launchComponentProcess(*name, component, args[1:])
			if err != nil {
				if p != nil && p.Pid != 0 {
					fmt.Printf("Error occured, but the process may be already started with PID %d\n", p.Pid)
				}
				return err
			}
			fmt.Printf("Started %s %s...\n", p.Exec, strings.Join(p.Args, " "))
			fmt.Printf("Process %d started for %s\n", p.Pid, component)
			return nil
		},
	}
	return cmd
}

func launchComponentProcess(name, spec string, args []string) (*compProcess, error) {
	component, version := meta.ParseCompVersion(spec)
	binPath, err := getServerBinPath(component, version)
	if err != nil {
		return nil, err
	}

	profileDir := profile.Root()
	p := &compProcess{
		Exec: binPath,
		Args: args,
		Dir:  path.Join(profileDir, "data", component, name),
		Env: []string{
			"TIUP_HOME=" + profileDir,
			"TIUP_INSTANCE=" + name,
		},
	}

	//fmt.Printf("%s %s\n", binPath, args)
	if err := p.Launch(false); err != nil {
		return p, err
	}

	return p, saveProcessToList(p)
}

func getServerBinPath(component string, version meta.Version) (string, error) {
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

	needDownload := false
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
		manifest, err := repository.ComponentVersions(component)
		if err != nil {
			return "", errors.Trace(err)
		}
		err = profile.SaveVersions(component, manifest)
		if err != nil {
			return "", errors.Trace(err)
		}
		if version.IsEmpty() {
			version = manifest.LatestStable()
		}
		compDir := profile.ComponentsDir()
		spec := fmt.Sprintf("%s:%s", component, version)
		err = repository.DownloadComponent(compDir, spec, false)
		if err != nil {
			return "", errors.Trace(err)
		}
	}

	return profile.BinaryPath(component, version)
}

type compProcess struct {
	Pid  int      `json:"pid,omitempty"`  // PID of the process
	Exec string   `json:"exec,omitempty"` // Path to the binary
	Args []string `json:"args,omitempty"` // Command line arguments
	Env  []string `json:"env,omitempty"`  // Enviroment variables
	Dir  string   `json:"dir,omitempty"`  // Working directory
}

type compProcessList []compProcess

// Launch executes the process
func (p *compProcess) Launch(async bool) error {
	dir := utils.MustDir(p.Dir)
	c, err := utils.Exec(os.Stdout, os.Stderr, dir, p.Exec, p.Args, p.Env)
	if err != nil {
		return err
	}
	p.Pid = c.Process.Pid
	if !async {
		return c.Wait()
	}
	return nil
}

func getProcessList() (compProcessList, error) {
	var list compProcessList
	var err error

	profile.ReadJSON(processListFilename, &list)
	return list, err
}

func saveProcessList(pl *compProcessList) error {
	return profile.WriteJSON(processListFilename, pl)
}

func saveProcessToList(p *compProcess) error {
	currList, err := getProcessList()
	if err != nil {
		return err
	}

	for _, currProc := range currList {
		if currProc.Pid == p.Pid {
			return fmt.Errorf("process %d already exist", p.Pid)
		}
	}

	newList := append(currList, *p)
	return saveProcessList(&newList)
}

func newProcListCmd() *cobra.Command {
	cmdProcList := &cobra.Command{
		Use:   "list",
		Short: "Show process list",
		Long: `Show current process list, note that this is the list saved when
the process launched, the actual process might already exited and no longer running.`,
		RunE: showProcessList,
	}
	return cmdProcList
}

func showProcessList(cmd *cobra.Command, args []string) error {
	procList, err := getProcessList()
	if err != nil {
		return err
	}

	fmt.Println("Launched processes:")
	var procTable [][]string
	procTable = append(procTable, []string{"Process", "PID", "Working Dir", "Argument"})
	for _, proc := range procList {
		procTable = append(procTable, []string{
			filepath.Base(proc.Exec),
			fmt.Sprint(proc.Pid),
			proc.Dir,
			strings.Join(proc.Args, " "),
		})
	}

	tui.PrintTable(procTable, true)
	return nil
}
