package main

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/pingcap/tiup/pkg/tui/colorstr"
	tuiterm "github.com/pingcap/tiup/pkg/tui/term"
	"github.com/pingcap/tiup/pkg/utils"
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

	if len(plan.Downloads) > 0 {
		tokens.Fprintf(&b, "[light_magenta]==> [reset][bold]Download Packages:[reset]\n")
		for _, d := range plan.Downloads {
			if d.ComponentID == "" || d.ResolvedVersion == "" {
				continue
			}
			tokens.Fprintf(&b, "  [green]+[reset] %s[dark_gray]@%s[reset]\n", d.ComponentID, d.ResolvedVersion)
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
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		tokens.Fprintf(&b, "[light_magenta]==> [reset][bold]Existing Packages:[reset]\n")
		for _, c := range reusedComponents {
			componentID, resolved, ok := strings.Cut(c, "@")
			if !ok || componentID == "" || resolved == "" {
				continue
			}
			tokens.Fprintf(&b, "    %s[dark_gray]@%s[reset]\n", componentID, resolved)
		}
	}

	if len(plan.Services) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		tokens.Fprintf(&b, "[light_magenta]==> [reset][bold]Start Services:[reset]\n")
		for _, s := range plan.Services {
			if s.ServiceID == "" || s.Name == "" {
				continue
			}

			componentHint := ""
			if componentID := strings.TrimSpace(s.ComponentID); componentID != "" && componentID != strings.TrimSpace(s.ServiceID) {
				componentHint = componentID
			}

			tokens.Fprintf(&b, "  [green]+[reset] ")
			if componentHint != "" {
				tokens.Fprintf(&b, "%s/", componentHint)
			}
			if ver := s.ResolvedVersion; ver != "" {
				tokens.Fprintf(&b, "%s[dark_gray]@%s[reset]", s.Name, ver)
			} else {
				tokens.Fprintf(&b, "%s", s.Name)
			}
			if binPath := strings.TrimSpace(s.BinPath); binPath != "" {
				tokens.Fprintf(&b, " [dark_gray](use %s)[reset]", binPath)
			}
			b.WriteString("\n")

			host := strings.TrimSpace(s.Shared.Host)
			if host != "" && s.Shared.Port > 0 {
				addr := utils.JoinHostPort(host, s.Shared.Port)
				if s.Shared.StatusPort > 0 && s.Shared.StatusPort != s.Shared.Port {
					addr = fmt.Sprintf("%s,%d", addr, s.Shared.StatusPort)
				}
				tokens.Fprintf(&b, "    [dark_gray]%s[reset]\n", addr)
			}

			if len(s.StartAfterServices) > 0 {
				tokens.Fprintf(&b, "    [dark_gray]Start after: %s[reset]\n", strings.Join(s.StartAfterServices, ","))
			}
		}
	}

	return b.String()
}
