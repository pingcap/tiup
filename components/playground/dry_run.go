package main

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

func writeDryRun(w io.Writer, plan BootPlan, format string) error {
	if w == nil {
		return fmt.Errorf("writer is nil")
	}
	format = strings.TrimSpace(strings.ToLower(format))
	if format == "" {
		format = "text"
	}

	switch format {
	case "text":
		_, err := io.WriteString(w, renderDryRunText(plan))
		return err
	case "json":
		data, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "%s\n", data)
		return err
	default:
		return fmt.Errorf("unknown --dry-run-output %q (expected text|json)", format)
	}
}

func renderDryRunText(plan BootPlan) string {
	var b strings.Builder

	fmt.Fprintf(&b, "DataDir: %s\n", plan.DataDir)
	fmt.Fprintf(&b, "Version: %s\n", plan.BootVersion)
	fmt.Fprintf(&b, "Host: %s\n", plan.Host)
	fmt.Fprintf(&b, "Mode: %s\n", plan.Shared.Mode)
	fmt.Fprintf(&b, "PDMode: %s\n", plan.Shared.PDMode)
	fmt.Fprintf(&b, "PortOffset: %d\n", plan.Shared.PortOffset)
	fmt.Fprintf(&b, "Monitor: %t\n", plan.Monitor)

	if len(plan.Downloads) > 0 {
		b.WriteString("\nDownloads:\n")
		for _, d := range plan.Downloads {
			if d.ComponentID == "" || d.ResolvedVersion == "" {
				continue
			}
			fmt.Fprintf(&b, "- Install %s@%s", d.ComponentID, d.ResolvedVersion)
			if d.DebugReason != "" {
				fmt.Fprintf(&b, " reason=%s", d.DebugReason)
			}
			if d.DebugBinPath != "" {
				fmt.Fprintf(&b, " binpath=%s", d.DebugBinPath)
			}
			if d.DebugSourceURL != "" {
				fmt.Fprintf(&b, " source=%s", d.DebugSourceURL)
			}
			if d.DebugInstallDir != "" {
				fmt.Fprintf(&b, " install_dir=%s", d.DebugInstallDir)
			}
			if d.DebugConstraint != "" {
				fmt.Fprintf(&b, " constraint=%s", d.DebugConstraint)
			}
			b.WriteString("\n")
		}
	}

	var reusedComponents []string
	if len(plan.Services) > 0 {
		downloaded := make(map[string]struct{}, len(plan.Downloads))
		for _, d := range plan.Downloads {
			if d.ComponentID == "" || d.ResolvedVersion == "" {
				continue
			}
			downloaded[d.ComponentID+"@"+d.ResolvedVersion] = struct{}{}
		}

		seen := make(map[string]struct{})
		for _, s := range plan.Services {
			if s.BinPath != "" || s.ComponentID == "" || s.ResolvedVersion == "" {
				continue
			}
			key := s.ComponentID + "@" + s.ResolvedVersion
			if _, ok := downloaded[key]; ok {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			reusedComponents = append(reusedComponents, key)
		}
		slices.Sort(reusedComponents)
	}

	if len(reusedComponents) > 0 {
		b.WriteString("\nReused:\n")
		for _, c := range reusedComponents {
			fmt.Fprintf(&b, "- Reuse %s\n", c)
		}
	}

	if len(plan.Services) > 0 {
		b.WriteString("\nServices:\n")
		for _, s := range plan.Services {
			if s.ServiceID == "" || s.Name == "" {
				continue
			}
			fmt.Fprintf(&b, "- Start %s(%s): host=%s port=%d status_port=%d dir=%s",
				s.ServiceID,
				s.Name,
				s.Shared.Host,
				s.Shared.Port,
				s.Shared.StatusPort,
				s.Shared.Dir,
			)
			if s.BinPath != "" {
				fmt.Fprintf(&b, " binpath=%s", s.BinPath)
			} else if s.ComponentID != "" && s.ResolvedVersion != "" {
				fmt.Fprintf(&b, " component=%s@%s", s.ComponentID, s.ResolvedVersion)
			}
			if s.Shared.ConfigPath != "" {
				fmt.Fprintf(&b, " config=%s", s.Shared.ConfigPath)
			}
			if s.Shared.UpTimeout > 0 {
				fmt.Fprintf(&b, " timeout=%ds", s.Shared.UpTimeout)
			}
			if len(s.StartAfterServices) > 0 {
				fmt.Fprintf(&b, " start_after=%s", strings.Join(s.StartAfterServices, ","))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}
