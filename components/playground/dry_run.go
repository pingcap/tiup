package main

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/tui/colorstr"
	tuiterm "github.com/pingcap/tiup/pkg/tui/term"
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
		_, err := io.WriteString(w, renderDryRunText(w, plan))
		return err
	case "json":
		redacted := plan
		if strings.TrimSpace(redacted.Shared.CSE.AccessKey) != "" {
			redacted.Shared.CSE.AccessKey = "***"
		}
		if strings.TrimSpace(redacted.Shared.CSE.SecretKey) != "" {
			redacted.Shared.CSE.SecretKey = "***"
		}

		data, err := json.MarshalIndent(redacted, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "%s\n", data)
		return err
	default:
		return fmt.Errorf("unknown --dry-run-output %q (expected text|json)", format)
	}
}

func renderDryRunText(out io.Writer, plan BootPlan) string {
	var b strings.Builder
	if out == nil {
		out = io.Discard
	}

	tokens := colorstr.DefaultTokens
	tokens.Disable = !tuiterm.Resolve(out).Color

	tokens.Fprintf(&b, "[bold]TiUP Playground dry-run plan[reset]\n")

	tokens.Fprintf(&b, "[dark_gray]DataDir:[reset] %s\n", plan.DataDir)
	tokens.Fprintf(&b, "[dark_gray]Version:[reset] %s\n", plan.BootVersion)
	tokens.Fprintf(&b, "[dark_gray]Host:[reset] %s\n", plan.Host)
	tokens.Fprintf(&b, "[dark_gray]Mode:[reset] %s\n", plan.Shared.Mode)
	tokens.Fprintf(&b, "[dark_gray]PDMode:[reset] %s\n", plan.Shared.PDMode)
	tokens.Fprintf(&b, "[dark_gray]PortOffset:[reset] %d\n", plan.Shared.PortOffset)
	tokens.Fprintf(&b, "[dark_gray]HighPerf:[reset] %t\n", plan.Shared.HighPerf)
	tokens.Fprintf(&b, "[dark_gray]EnableTiKVColumnar:[reset] %t\n", plan.Shared.EnableTiKVColumnar)
	tokens.Fprintf(&b, "[dark_gray]ForcePull:[reset] %t\n", plan.Shared.ForcePull)
	tokens.Fprintf(&b, "[dark_gray]Monitor:[reset] %t\n", plan.Monitor)
	tokens.Fprintf(&b, "[dark_gray]GrafanaPort:[reset] %d\n", plan.GrafanaPort)

	switch plan.Shared.Mode {
	case proc.ModeCSE, proc.ModeDisAgg, proc.ModeNextGen:
		if endpoint := strings.TrimSpace(plan.Shared.CSE.S3Endpoint); endpoint != "" {
			tokens.Fprintf(&b, "[dark_gray]CSE.S3Endpoint:[reset] %s\n", endpoint)
		}
		if bucket := strings.TrimSpace(plan.Shared.CSE.Bucket); bucket != "" {
			tokens.Fprintf(&b, "[dark_gray]CSE.Bucket:[reset] %s\n", bucket)
		}
	}

	if len(plan.Downloads) > 0 {
		tokens.Fprintf(&b, "\n[bold]Downloads:[reset]\n")
		for _, d := range plan.Downloads {
			if d.ComponentID == "" || d.ResolvedVersion == "" {
				continue
			}
			tokens.Fprintf(&b, "- [yellow]Install[reset] %s@%s", d.ComponentID, d.ResolvedVersion)
			if d.DebugReason != "" {
				tokens.Fprintf(&b, " reason=%s", d.DebugReason)
			}
			if d.DebugBinPath != "" {
				tokens.Fprintf(&b, " binpath=%s", d.DebugBinPath)
			}
			if d.DebugSourceURL != "" {
				tokens.Fprintf(&b, " source=%s", d.DebugSourceURL)
			}
			if d.DebugInstallDir != "" {
				tokens.Fprintf(&b, " install_dir=%s", d.DebugInstallDir)
			}
			if d.DebugConstraint != "" {
				tokens.Fprintf(&b, " constraint=%s", d.DebugConstraint)
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
		tokens.Fprintf(&b, "\n[bold]Reused:[reset]\n")
		for _, c := range reusedComponents {
			tokens.Fprintf(&b, "- [dark_gray]Reuse[reset] %s\n", c)
		}
	}

	if len(plan.Services) > 0 {
		tokens.Fprintf(&b, "\n[bold]Services:[reset]\n")
		for _, s := range plan.Services {
			if s.ServiceID == "" || s.Name == "" {
				continue
			}
			tokens.Fprintf(&b, "- [green]Start[reset] %s(%s): host=%s port=%d status_port=%d dir=%s",
				s.ServiceID,
				s.Name,
				s.Shared.Host,
				s.Shared.Port,
				s.Shared.StatusPort,
				s.Shared.Dir,
			)
			if s.BinPath != "" {
				tokens.Fprintf(&b, " binpath=%s", s.BinPath)
			} else if s.ComponentID != "" && s.ResolvedVersion != "" {
				tokens.Fprintf(&b, " component=%s@%s", s.ComponentID, s.ResolvedVersion)
			}
			if s.Shared.ConfigPath != "" {
				tokens.Fprintf(&b, " config=%s", s.Shared.ConfigPath)
			}
			if s.Shared.UpTimeout > 0 {
				tokens.Fprintf(&b, " timeout=%ds", s.Shared.UpTimeout)
			}
			if len(s.StartAfterServices) > 0 {
				tokens.Fprintf(&b, " start_after=%s", strings.Join(s.StartAfterServices, ","))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}
