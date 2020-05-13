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

package cmd

import (
	"fmt"

	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/spf13/cobra"
)

func newRepoGenkeyCmd(env *meta.Environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "genkey",
		Short: "Generate a new key pair",
		Long:  `Generate a new key pair that can be used to sign components.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			pubKey, privKey, err := genKeyPair()
			if err != nil {
				return err
			}
			// TODO: save keys to files
			fmt.Printf("Private key generated:\n%s\n", privKey)
			fmt.Printf("Public key of the private key:\n%s\n", pubKey)
			return nil
		},
	}

	return cmd
}

func genKeyPair() ([]byte, []byte, error) {
	// TODO
	return []byte("public key"), []byte("private key"), nil
}
