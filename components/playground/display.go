package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/utils"
)

func (p *Playground) buildProcTitleCounts() map[string]int {
	counts := make(map[string]int)
	if p == nil {
		return counts
	}
	_ = p.WalkProcs(func(_ proc.ServiceID, ins proc.Process) error {
		if ins == nil {
			return nil
		}
		counts[procTitle(ins)]++
		return nil
	})
	return counts
}

type displayItem struct {
	Name      string `json:"name"`
	ServiceID string `json:"service"`
	Component string `json:"component,omitempty"`
	Addr      string `json:"addr,omitempty"`
	Status    string `json:"status"`
	Uptime    string `json:"uptime,omitempty"`

	PID     int    `json:"pid,omitempty"`
	Version string `json:"version,omitempty"`
	Binary  string `json:"binary,omitempty"`
	Log     string `json:"log,omitempty"`
}

func (p *Playground) handleDisplay(state *controllerState, r io.Writer, verbose, jsonOut bool) error {
	if p == nil {
		return fmt.Errorf("playground is nil")
	}
	if state == nil {
		return fmt.Errorf("playground controller state is nil")
	}
	if r == nil {
		r = io.Discard
	}

	type addrGetter interface {
		Addr() string
	}

	collect := func(serviceID proc.ServiceID, ins proc.Process) (*displayItem, error) {
		if ins == nil {
			return nil, nil
		}

		info := ins.Info()
		if info == nil {
			return nil, nil
		}

		pid := 0
		status := "not started"
		uptime := ""

		if proc := info.Proc; proc != nil {
			cmd := proc.Cmd()
			if cmd != nil && cmd.Process != nil {
				pid = proc.Pid()
				uptime = proc.Uptime()
				status = "running"
				if ps := cmd.ProcessState; ps != nil {
					status = fmt.Sprintf("exited(%d)", ps.ExitCode())
				}
			}
		}

		addr := ""
		if v, ok := ins.(addrGetter); ok {
			addr = v.Addr()
		}

		item := &displayItem{
			Name:      info.Name(),
			ServiceID: serviceID.String(),
			Addr:      addr,
			Status:    status,
			Uptime:    uptime,
		}
		if verbose {
			item.PID = pid
			item.Version = info.Version.String()
			item.Binary = info.BinPath
			item.Log = ins.LogFile()
			item.Component = info.RepoComponentID.String()
		}
		return item, nil
	}

	if jsonOut {
		var items []*displayItem
		err := state.walkProcs(func(serviceID proc.ServiceID, ins proc.Process) error {
			item, err := collect(serviceID, ins)
			if err != nil {
				return err
			}
			if item == nil {
				return nil
			}
			items = append(items, item)
			return nil
		})
		if err != nil {
			return err
		}

		enc := json.NewEncoder(r)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	}

	header := []string{"NAME", "SERVICE", "ADDR", "STATUS", "UPTIME"}
	if verbose {
		header = []string{"NAME", "SERVICE", "COMPONENT", "ADDR", "STATUS", "UPTIME", "PID", "VERSION", "BINARY", "LOG"}
	}
	td := utils.NewTableDisplayer(r, header)

	if err := state.walkProcs(func(serviceID proc.ServiceID, ins proc.Process) error {
		item, err := collect(serviceID, ins)
		if err != nil || item == nil {
			return err
		}

		if !verbose {
			td.AddRow(item.Name, item.ServiceID, item.Addr, item.Status, item.Uptime)
			return nil
		}

		binary := item.Binary
		if info := ins.Info(); info != nil && info.UserBinPath != "" {
			binary = info.UserBinPath
		}

		td.AddRow(
			item.Name,
			item.ServiceID,
			item.Component,
			item.Addr,
			item.Status,
			item.Uptime,
			strconv.Itoa(item.PID),
			item.Version,
			prettifyUserPath(binary),
			prettifyUserPath(item.Log),
		)
		return nil
	}); err != nil {
		return err
	}

	td.Display()
	return nil
}

func procTitle(inst proc.Process) string {
	if inst == nil {
		return "Instance"
	}
	info := inst.Info()
	if info == nil {
		return "Instance"
	}
	if name := info.DisplayName(); name != "" {
		return name
	}
	if name := info.Service.String(); name != "" {
		return name
	}
	return "Instance"
}

func procDisplayName(inst proc.Process, includeID bool) string {
	if inst == nil {
		return "Instance"
	}

	title := procTitle(inst)
	if includeID {
		info := inst.Info()
		if info == nil {
			return title
		}
		return fmt.Sprintf("%s %d", title, info.ID)
	}
	return title
}

func prettifyUserPath(p string) string {
	if p == "" {
		return p
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	home = filepath.Clean(home)
	p = filepath.Clean(p)

	if p == home {
		return "~"
	}
	sep := string(os.PathSeparator)
	if strings.HasPrefix(p, home+sep) {
		return "~" + sep + strings.TrimPrefix(p, home+sep)
	}
	return p
}
