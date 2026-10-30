// Copyright 2026 PingCAP, Inc.
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
	"os"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
)

// This local-only entry deliberately does not register or replace a mirror
// component. Every other TiUP component keeps its normal resolution path.
func newDiagRedactCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "diag-redact [args...]",
		Short:              "Run the local diag-redact executable beside this TiUP binary",
		DisableFlagParsing: true,
		RunE: func(_ *cobra.Command, args []string) error {
			driver, err := os.Executable()
			if err != nil {
				return err
			}
			binary, err := localDiagRedactPath(driver)
			if err != nil {
				return err
			}
			if len(args) > 0 && args[0] == "--" {
				args = args[1:]
			}
			// Replace the driver process so signals, exit status, terminal and
			// working directory pass through without a background relay task.
			return syscall.Exec(binary, append([]string{binary}, args...), os.Environ())
		},
	}
}

func localDiagRedactPath(driver string) (string, error) {
	resolved, err := filepath.EvalSymlinks(driver)
	if err != nil {
		return "", err
	}
	binary := filepath.Join(filepath.Dir(resolved), "diag-redact")
	info, err := os.Stat(binary)
	if err != nil {
		return "", fmt.Errorf("local diag-redact not found beside TiUP; keep both executables in the same bundle directory: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", fmt.Errorf("local diag-redact is not a regular executable: %s", binary)
	}
	return binary, nil
}
