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

package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository"
	gops "github.com/shirou/gopsutil/process"
	"github.com/spf13/cobra"
)

func main() {
	if err := execute(); err != nil {
		os.Exit(1)
	}
}

func execute() error {
	rootCmd := &cobra.Command{
		Use:          "tiup client",
		Short:        "Connect a TiDB cluster in your local host",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) > 0 {
				target = args[0]
			}
			env, err := environment.InitEnv(repository.Options{}, repository.MirrorOptions{})
			if err != nil {
				return err
			}
			environment.SetGlobalEnv(env)
			return connect(target)
		},
		PostRunE: func(cmd *cobra.Command, args []string) error {
			return environment.GlobalEnv().Close()
		},
	}

	return rootCmd.Execute()
}

func connect(target string) error {
	tiupHome := os.Getenv(localdata.EnvNameHome)
	if tiupHome == "" {
		return fmt.Errorf("env variable %s not set, are you running client out of tiup?", localdata.EnvNameHome)
	}
	endpoints, err := scanEndpoint(tiupHome)
	if err != nil {
		return fmt.Errorf("error on read files: %s", err.Error())
	}
	if len(endpoints) == 0 {
		return fmt.Errorf("It seems no playground is running, execute `tiup playground` to start one")
	}
	var ep *endpoint
	if target == "" {
		if ep = selectEndpoint(endpoints); ep == nil {
			os.Exit(0)
		}
	} else {
		for _, end := range endpoints {
			if end.component == target {
				ep = end
			}
		}
		if ep == nil {
			return fmt.Errorf("specified instance %s not found, maybe it's not alive now, execute `tiup status` to see instance list", target)
		}
	}
	fmt.Printf("MySQL Shell:  mysqlsh %s\n", ep.dsn)
	r := regexp.MustCompile(`^mysql://([a-z]+)@(.+):(\d+)`)
	m := r.FindStringSubmatch(ep.dsn)
	if m != nil {
		fmt.Printf("MySQL Client: mysql -u %s -h %s -P %s\n", m[1], m[2], m[3])
	}
	return nil
}

func scanEndpoint(tiupHome string) ([]*endpoint, error) {
	endpoints := []*endpoint{}

	files, err := os.ReadDir(path.Join(tiupHome, localdata.DataParentDir))
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !isInstanceAlive(tiupHome, file.Name()) {
			continue
		}
		endpoints = append(endpoints, readDsn(path.Join(tiupHome, localdata.DataParentDir, file.Name()), file.Name())...)
	}
	return endpoints, nil
}

func isInstanceAlive(tiupHome, instance string) bool {
	s, err := environment.GlobalEnv().Profile().ReadMetaFile(instance)
	if err != nil {
		return false
	}
	if s == nil {
		return false
	}
	exist, _ := gops.PidExists(int32(s.Pid))
	return exist
}

func readDsn(dir, component string) []*endpoint {
	endpoints := []*endpoint{}

	file, err := os.Open(path.Join(dir, "dsn"))
	if err != nil {
		return endpoints
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		endpoints = append(endpoints, &endpoint{
			component: component,
			dsn:       scanner.Text(),
		})
	}

	return endpoints
}

func selectEndpoint(endpoints []*endpoint) *endpoint {
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	l := widgets.NewList()
	l.Title = "Choose an endpoint to connect"

	ml := 0
	for _, ep := range endpoints {
		if ml < len(ep.component) {
			ml = len(ep.component)
		}
	}
	fmtStr := fmt.Sprintf(" %%-%ds %%s", ml)
	for _, ep := range endpoints {
		l.Rows = append(l.Rows, fmt.Sprintf(fmtStr, ep.component, ep.dsn))
	}
	l.TextStyle = ui.NewStyle(ui.ColorWhite)
	l.SelectedRowStyle = ui.NewStyle(ui.ColorGreen)
	l.WrapText = false
	size := min(len(endpoints), 16)
	l.SetRect(0, 0, 80, size+2)

	ui.Render(l)

	uiEvents := ui.PollEvents()
	for {
		e := <-uiEvents
		_ = os.WriteFile("/tmp/log", []byte(e.ID+"\n"), 0o664)
		switch e.ID {
		case "q", "<C-c>":
			return nil
		case "j", "<Down>":
			l.ScrollDown()
		case "k", "<Up>":
			l.ScrollUp()
		case "<Enter>":
			return endpoints[l.SelectedRow]
		}

		ui.Render(l)
	}
}
