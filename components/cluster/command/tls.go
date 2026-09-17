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
	"github.com/pingcap/tiup/pkg/cluster/manager"
	"github.com/spf13/cobra"
)

func newTLSCmd() *cobra.Command {
	var (
		reloadCertificate bool // reload certificate when the cluster enable encrypted communication
		cleanCertificate  bool // cleanup certificate when the cluster disable encrypted communication
		enableTLS         bool
		customMode        bool
		clientCA          string
		clientCert        string
		clientKey         string
	)

	cmd := &cobra.Command{
		Use:   "tls <cluster-name> <enable/disable/swap-client-cert>",
		Short: "Enable/Disable TLS between TiDB components",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			if err := validRoles(gOpt.Roles); err != nil {
				return err
			}
			clusterName := args[0]

			switch strings.ToLower(args[1]) {
			case "enable":
				enableTLS = true
			case "disable":
				enableTLS = false
			case "swap-client-cert":
				if clientCA == "" || clientCert == "" || clientKey == "" {
					return perrs.New("swap-client-cert requires --client-ca, --client-cert, and --client-key")
				}
				return cm.SwapClientCert(clusterName, clientCA, clientCert, clientKey)
			default:
				return perrs.New("action must be one of: enable, disable, swap-client-cert")
			}

			if enableTLS && cleanCertificate {
				return perrs.New("clean-certificate only works when tls disable")
			}

			if !enableTLS && reloadCertificate {
				return perrs.New("reload-certificate only works when tls enable")
			}

			if !enableTLS && customMode {
				return perrs.New("custom mode only applies to enable")
			}

			if customMode && (clientCA == "" || clientCert == "" || clientKey == "") {
				return perrs.New("--custom requires --client-ca, --client-cert, and --client-key")
			}

			customOpts := manager.CustomTLSOptions{
				Enabled:    customMode,
				ClientCA:   clientCA,
				ClientCert: clientCert,
				ClientKey:  clientKey,
			}

			return cm.TLS(clusterName, gOpt, enableTLS, cleanCertificate, reloadCertificate, skipConfirm, customOpts)
		},
	}

	cmd.Flags().BoolVar(&cleanCertificate, "clean-certificate", false, "Cleanup the certificate file if it already exists when tls disable")
	cmd.Flags().BoolVar(&reloadCertificate, "reload-certificate", false, "Load the certificate file whether it exists or not when tls enable")
	cmd.Flags().BoolVar(&gOpt.Force, "force", false, "Force enable/disable tls regardless of the current state")
	cmd.Flags().BoolVar(&customMode, "custom", false, "Use custom (BYOC) certificates instead of TiUP-managed self-signed certificates")
	cmd.Flags().StringVar(&clientCA, "client-ca", "", "Path to the client CA certificate file (used with --custom)")
	cmd.Flags().StringVar(&clientCert, "client-cert", "", "Path to the client certificate file (used with --custom)")
	cmd.Flags().StringVar(&clientKey, "client-key", "", "Path to the client private key file (used with --custom)")

	return cmd
}
