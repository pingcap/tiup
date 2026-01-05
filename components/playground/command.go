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
	"bytes"
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
	"github.com/pingcap/tiup/pkg/tui/colorstr"
	"github.com/spf13/cobra"

	tuiv2output "github.com/pingcap/tiup/pkg/tuiv2/output"
)

type playgroundNotRunningError struct {
	err error
}

func (e playgroundNotRunningError) Error() string {
	if e.err == nil {
		return "no playground running"
	}
	return e.err.Error()
}

func (e playgroundNotRunningError) Unwrap() error { return e.err }

func isPlaygroundNotRunning(err error) bool {
	var notRunning playgroundNotRunningError
	return stdErrors.As(err, &notRunning)
}

type playgroundUnreachableError struct {
	err error
}

func (e playgroundUnreachableError) Error() string {
	if e.err == nil {
		return "playground is unreachable"
	}
	return e.err.Error()
}

func (e playgroundUnreachableError) Unwrap() error { return e.err }

func isPlaygroundUnreachable(err error) bool {
	var unreachable playgroundUnreachableError
	return stdErrors.As(err, &unreachable)
}

func shouldSuggestPlaygroundNotRunning(err error) bool {
	if err == nil {
		return false
	}
	if isPlaygroundNotRunning(err) {
		return true
	}
	// "Connection refused" for the local HTTP command server is a strong signal
	// that the playground isn't running.
	return stdErrors.Is(err, syscall.ECONNREFUSED)
}

// CommandType send to playground.
type CommandType string

// types of CommandType
const (
	ScaleInCommandType  CommandType = "scale-in"
	ScaleOutCommandType CommandType = "scale-out"
	DisplayCommandType  CommandType = "display"
)

type DisplayRequest struct {
	Verbose bool `json:"verbose,omitempty"`
	JSON    bool `json:"json,omitempty"`
}

type ScaleInRequest struct {
	Name string `json:"name,omitempty"`
	PID  int    `json:"pid,omitempty"`
}

type ScaleOutRequest struct {
	ServiceID proc.ServiceID `json:"service"`
	Count     int            `json:"count"`
	Config    proc.Config    `json:"config"`
}

// Command sends a request to a running playground via its HTTP control server.
type Command struct {
	Type     CommandType      `json:"type"`
	Display  *DisplayRequest  `json:"display,omitempty"`
	ScaleIn  *ScaleInRequest  `json:"scale_in,omitempty"`
	ScaleOut *ScaleOutRequest `json:"scale_out,omitempty"`
}

// CommandReply is the (optional) structured response returned by the playground
// command server when the client asks for JSON output.
type CommandReply struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// cliState holds process-level CLI state for both "tiup playground" (boot) and
// its subcommands (display/scale-in/scale-out).
//
// It intentionally avoids package-level mutable globals so tests and helpers can
// be pure and side-effect free.
type cliState struct {
	options BootOptions

	tag            string
	tiupDataDir    string
	dataDir        string
	deleteWhenExit bool
}

func newCLIState() *cliState {
	return &cliState{options: BootOptions{Monitor: true}}
}

func scaleOutServiceIDs() []proc.ServiceID {
	var out []proc.ServiceID
	for _, spec := range pgservice.AllSpecs() {
		if spec.ServiceID == "" || !spec.Catalog.AllowScaleOut {
			continue
		}
		out = append(out, spec.ServiceID)
	}
	slices.SortFunc(out, func(a, b proc.ServiceID) int {
		return strings.Compare(a.String(), b.String())
	})
	return out
}

func newScaleOut(state *cliState) *cobra.Command {
	var services []string
	var counts []int
	var cfg proc.Config
	var legacy *legacyScaleOutFlags

	supported := scaleOutServiceIDs()
	supportedText := "Service ID(s) to scale out (repeatable)"
	if len(supported) > 0 {
		var ids []string
		for _, id := range supported {
			ids = append(ids, id.String())
		}
		supportedText += " (supported: " + strings.Join(ids, ", ") + ")"
	}

	cmd := &cobra.Command{
		Use:     "scale-out",
		Short:   "Scale out instances in a running playground",
		Example: "tiup playground scale-out --service tidb --count 1",
		RunE: func(cmd *cobra.Command, args []string) error {
			var reqs []ScaleOutRequest
			switch {
			case len(services) > 0:
				if legacy != nil && legacy.hasCount() {
					return fmt.Errorf("cannot mix --service with legacy component flags (e.g. --db/--kv)")
				}
				if len(services) != len(counts) {
					return fmt.Errorf("--service and --count must have the same length")
				}
				for i, raw := range services {
					serviceID := proc.ServiceID(strings.TrimSpace(raw))
					if serviceID == "" {
						continue
					}
					spec, ok := pgservice.SpecFor(serviceID)
					if !ok {
						return fmt.Errorf("unknown service %s", serviceID)
					}
					if !spec.Catalog.AllowScaleOut {
						return fmt.Errorf("service %q does not support scale-out", serviceID)
					}
					count := counts[i]
					if count <= 0 {
						return fmt.Errorf("scale-out count must be greater than 0")
					}
					reqs = append(reqs, ScaleOutRequest{
						ServiceID: serviceID,
						Count:     count,
						Config:    cfg,
					})
				}
			default:
				var err error
				reqs, err = legacy.requests()
				if err != nil {
					return err
				}
				if len(reqs) == 0 {
					return cmd.Help()
				}
			}

			num, err := scaleOut(cmd.OutOrStdout(), reqs, state)
			if err != nil {
				return err
			}

			if num == 0 {
				return cmd.Help()
			}

			return nil
		},
		Hidden: false,
	}

	cmd.Flags().StringSliceVar(&services, "service", nil, supportedText)
	cmd.Flags().IntSliceVar(&counts, "count", nil, "Instance count(s) for each --service (repeatable)")
	cmd.Flags().StringVar(&cfg.Host, "host", "", "Host for new instances (default: inherit from boot config)")
	cmd.Flags().IntVar(&cfg.Port, "port", 0, "Port for new instances (0 means default)")
	cmd.Flags().StringVar(&cfg.ConfigPath, "config", "", "Config file for new instances (default: inherit from boot config)")
	cmd.Flags().StringVar(&cfg.BinPath, "binpath", "", "Binary path for new instances (default: inherit from boot config)")
	cmd.Flags().IntVar(&cfg.UpTimeout, "timeout", 0, "Max wait time in seconds for starting, 0 means no limit")

	// LEGACY: tiup playground scale-out --db 1 --kv 2 ...
	legacy = registerLegacyScaleOutFlags(cmd)

	return cmd
}

func newScaleIn(state *cliState) *cobra.Command {
	var names []string
	var pids []int

	cmd := &cobra.Command{
		Use:     "scale-in",
		Short:   "Scale in one or more instances by name or pid",
		Example: "  tiup playground scale-in --name tidb-0\n  tiup playground scale-in --pid 12345",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(names) == 0 && len(pids) == 0 {
				return cmd.Help()
			}
			if len(names) > 0 && len(pids) > 0 {
				out := tuiv2output.Stderr.Get()
				fmt.Fprint(out, tuiv2output.Callout{
					Style:   tuiv2output.CalloutFailed,
					Content: "scale-in expects exactly one of --name or --pid",
				}.Render(out))
				return renderedError{err: fmt.Errorf("scale-in expects exactly one of --name or --pid")}
			}

			reqs := make([]ScaleInRequest, 0, max(len(names), len(pids)))
			if len(names) > 0 {
				for _, name := range names {
					name = strings.TrimSpace(name)
					if name == "" {
						continue
					}
					reqs = append(reqs, ScaleInRequest{Name: name})
				}
			} else {
				for _, pid := range pids {
					if pid <= 0 {
						return fmt.Errorf("--pid must be greater than 0")
					}
					reqs = append(reqs, ScaleInRequest{PID: pid})
				}
			}
			if len(reqs) == 0 {
				return cmd.Help()
			}
			return scaleIn(cmd.OutOrStdout(), reqs, state)
		},
		Hidden: false,
	}

	cmd.Flags().StringSliceVar(&names, "name", nil, "Instance name(s) to scale in (get from tiup playground display)")
	cmd.Flags().IntSliceVar(&pids, "pid", nil, "Instance PID(s) to scale in (get from tiup playground display --verbose)")

	return cmd
}

func newDisplay(state *cliState) *cobra.Command {
	var verbose bool
	var jsonOut bool
	cmd := &cobra.Command{
		Use:    "display",
		Short:  "Display instances in the running playground",
		Hidden: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := display(cmd.OutOrStdout(), verbose, jsonOut, state); err != nil {
				return err
			}
			if !verbose && !jsonOut {
				colorstr.Fprintf(tuiv2output.Stderr.Get(), "\n[dark_gray]Tip: use --verbose to show more columns: COMPONENT, PID, VERSION, BINARY, LOG[reset]\n")
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show more details for each instance")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output in JSON format")
	return cmd
}

func scaleIn(out io.Writer, reqs []ScaleInRequest, state *cliState) error {
	target, err := resolvePlaygroundTarget(state.tag, state.tiupDataDir, state.dataDir)
	if err != nil {
		printDisplayFailureWarning(out, err)
		return renderedError{err: err}
	}

	var cmds []Command
	for _, req := range reqs {
		req := req
		if req.Name == "" && req.PID <= 0 {
			continue
		}
		c := Command{
			Type:    ScaleInCommandType,
			ScaleIn: &req,
		}
		cmds = append(cmds, c)
	}

	addr := "127.0.0.1:" + strconv.Itoa(target.port)
	if err := sendCommandsAndPrintResult(out, cmds, addr); err != nil {
		printDisplayFailureWarning(out, err)
		return renderedError{err: err}
	}
	return nil
}

func scaleOut(out io.Writer, reqs []ScaleOutRequest, state *cliState) (num int, err error) {
	target, err := resolvePlaygroundTarget(state.tag, state.tiupDataDir, state.dataDir)
	if err != nil {
		printDisplayFailureWarning(out, err)
		return 0, renderedError{err: err}
	}

	if len(reqs) == 0 {
		return 0, nil
	}

	cmds := make([]Command, 0, len(reqs))
	for _, req := range reqs {
		req := req
		cmds = append(cmds, Command{
			Type:     ScaleOutCommandType,
			ScaleOut: &req,
		})
	}

	addr := "127.0.0.1:" + strconv.Itoa(target.port)
	if err := sendCommandsAndPrintResult(out, cmds, addr); err != nil {
		printDisplayFailureWarning(out, err)
		return 0, renderedError{err: err}
	}
	return len(cmds), nil
}

func display(out io.Writer, verbose, jsonOut bool, state *cliState) error {
	target, err := resolvePlaygroundTarget(state.tag, state.tiupDataDir, state.dataDir)
	if err != nil {
		printDisplayFailureWarning(out, err)
		return renderedError{err: err}
	}
	c := Command{
		Type:    DisplayCommandType,
		Display: &DisplayRequest{Verbose: verbose, JSON: jsonOut},
	}

	addr := "127.0.0.1:" + strconv.Itoa(target.port)
	if err := sendCommandsAndPrintResult(out, []Command{c}, addr); err != nil {
		printDisplayFailureWarning(out, err)
		return renderedError{err: err}
	}
	return nil
}

func printDisplayFailureWarning(out io.Writer, err error) {
	if err == nil || out == nil {
		return
	}

	var lines []string
	if shouldSuggestPlaygroundNotRunning(err) {
		lines = append(lines, colorstr.Sprintf("[bold]Looks like no TiUP Playground is running?[reset]"))
	}
	lines = append(lines, fmt.Sprintf("Error: %v", err))

	fmt.Fprint(out, tuiv2output.Callout{
		Style:   tuiv2output.CalloutWarning,
		Content: strings.Join(lines, "\n"),
	}.Render(out))
}

func sendCommandsAndPrintResult(out io.Writer, cmds []Command, addr string) error {
	if out == nil {
		out = io.Discard
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, cmd := range cmds {
		data, err := json.Marshal(&cmd)
		if err != nil {
			return errors.AddStack(err)
		}

		url := fmt.Sprintf("http://%s/command", addr)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			cancel()
			return errors.AddStack(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			cancel()
			return playgroundUnreachableError{err: err}
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		if readErr != nil {
			return errors.AddStack(readErr)
		}

		var reply CommandReply
		if err := json.Unmarshal(body, &reply); err != nil {
			return errors.Annotatef(err, "invalid command server response (status: %s)", resp.Status)
		}

		if reply.Message != "" {
			_, _ = io.WriteString(out, reply.Message)
		}
		// Only print server-side stderr output when the command is successful.
		// On failures, callers will render a single warning callout based on the
		// returned error to avoid duplicated messages.
		if reply.OK && reply.Error != "" {
			_, _ = io.WriteString(out, reply.Error)
			if reply.Error[len(reply.Error)-1] != '\n' {
				_, _ = io.WriteString(out, "\n")
			}
		}
		if !reply.OK {
			if reply.Error != "" {
				return errors.New(reply.Error)
			}
			return errors.Errorf("command failed (status: %s)", resp.Status)
		}
	}

	return nil
}
