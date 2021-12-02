// Copyright 2021 PingCAP, Inc.
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

package command

import (
	"strings"

	perrs "github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newTLSCmd() *cobra.Command {
	var (
		reloadCertificate bool // reload certificate when the cluster enable encrypted communication
		cleanCertificate  bool // cleanup certificate when the cluster disable encrypted communication
		enableTLS         bool
	)

	cmd := &cobra.Command{
		Use:   "tls <cluster-name> <enable/disable>",
		Short: "Enable/Disable TLS between TiDB components",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			if err := validRoles(gOpt.Roles); err != nil {
				return err
			}
			clusterName := args[0]
			clusterReport.ID = scrubClusterName(clusterName)
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			switch strings.ToLower(args[1]) {
			case "enable":
				enableTLS = true
			case "disable":
				enableTLS = false
			default:
				return perrs.New("enable or disable must be specified at least one")
			}

			if enableTLS && cleanCertificate {
				return perrs.New("clean-certificate only works when tls disable")
			}

			if !enableTLS && reloadCertificate {
				return perrs.New("reload-certificate only works when tls enable")
			}

			return cm.TLS(clusterName, gOpt, enableTLS, cleanCertificate, reloadCertificate, skipConfirm)
		},
	}

	cmd.Flags().BoolVar(&cleanCertificate, "clean-certificate", false, "Clean up the certificate file if it already exists when disable encrypted communication")
	cmd.Flags().BoolVar(&reloadCertificate, "reload-certificate", false, "Reload the certificate file if it already exists when enable encrypted communication")

	return cmd
}
