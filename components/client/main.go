package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"

	"github.com/c4pt0r/tiup/pkg/localdata"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
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
		Use:          "client",
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
		return fmt.Errorf("It seems no playground is running, execute `tiup run playground` to start one")
	}
	var endpoint *Endpoint
	if target == "" {
		if endpoint = selectEndpoint(endpoints); endpoint == nil {
			os.Exit(0)
		}
	} else {
		for _, end := range endpoints {
			if end.component == target {
				endpoint = end
			}
		}
		if endpoint == nil {
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
	if err = h.Open(endpoint.dsn); err != nil {
		return fmt.Errorf("can't open connection to %s: %s", endpoint.dsn, err.Error())
	}
	if err = h.Run(); err != io.EOF {
		return err
	}
	return nil
}

func scanEndpoint(tiupHome string) ([]*Endpoint, error) {
	endpoints := []*Endpoint{}

	files, err := ioutil.ReadDir(path.Join(tiupHome, "data"))
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		endpoints = append(endpoints, readDsn(path.Join(tiupHome, "data", file.Name()), file.Name())...)
	}
	return endpoints, nil
}

func readDsn(dir, component string) []*Endpoint {
	endpoints := []*Endpoint{}

	file, err := os.Open(path.Join(dir, "dsn"))
	if err != nil {
		return endpoints
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		endpoints = append(endpoints, &Endpoint{
			component: component,
			dsn:       scanner.Text(),
		})
	}

	return endpoints
}

func selectEndpoint(endpoints []*Endpoint) *Endpoint {
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	l := widgets.NewList()
	l.Title = "Choose a endpoint to connect"

	ml := 0
	for _, endpoint := range endpoints {
		if ml < len(endpoint.component) {
			ml = len(endpoint.component)
		}
	}
	fmtStr := fmt.Sprintf(" %%-%ds %%s", ml)
	for _, endpoint := range endpoints {
		l.Rows = append(l.Rows, fmt.Sprintf(fmtStr, endpoint.component, endpoint.dsn))
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
