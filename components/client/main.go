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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	gops "github.com/shirou/gopsutil/process"
	"github.com/spf13/cobra"
	_ "github.com/xo/usql/drivers/mysql"
	"github.com/xo/usql/env"
	"github.com/xo/usql/handler"
	"github.com/xo/usql/rline"
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
			return connect(target)
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
	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("can't get current user: %s", err.Error())
	}
	l, err := rline.New(false, "", env.HistoryFile(u))
	if err != nil {
		return fmt.Errorf("can't open history file: %s", err.Error())
	}
	h := handler.New(l, u, os.Getenv(localdata.EnvNameInstanceDataDir), true)
	if err = h.Open(ep.dsn); err != nil {
		return fmt.Errorf("can't open connection to %s: %s", ep.dsn, err.Error())
	}
	if err = h.Run(); err != io.EOF {
		return err
	}
	return nil
}

func scanEndpoint(tiupHome string) ([]*endpoint, error) {
	endpoints := []*endpoint{}

	files, err := ioutil.ReadDir(path.Join(tiupHome, localdata.DataParentDir))
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
	metaFile := path.Join(tiupHome, localdata.DataParentDir, instance, localdata.MetaFilename)

	// If the path doesn't contain the meta file, which means startup interrupted
	if utils.IsNotExist(metaFile) {
		return false
	}

	file, err := os.Open(metaFile)
	if err != nil {
		return false
	}
	defer file.Close()
	var process map[string]interface{}
	if err := json.NewDecoder(file).Decode(&process); err != nil {
		return false
	}

	if v, ok := process["pid"]; !ok {
		return false
	} else if pid, ok := v.(float64); !ok {
		return false
	} else if exist, err := gops.PidExists(int32(pid)); err != nil {
		return false
	} else {
		return exist
	}
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
	l.Title = "Choose a endpoint to connect"

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
	size := 16
	if len(endpoints) < size {
		size = len(endpoints)
	}
	l.SetRect(0, 0, 80, size+2)

	ui.Render(l)

	uiEvents := ui.PollEvents()
	for {
		e := <-uiEvents
		ioutil.WriteFile("/tmp/log", []byte(e.ID+"\n"), 0664)
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
