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
	"os"
	"time"

	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	repoPath string
)

func newRepoCmd(env *meta.Environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo <command>",
		Short: "Manage a repository for TiUP components",
		Long: `The 'repo' command is used to manage a component repository for TiUP, you can use
it to create a private repository, or to add new component to an existing repository.
The repository can be used either online or offline.
It also provides some useful utilities to help managing keys, users and versions
of components or the repository itself.`,
		Hidden: true, // WIP, remove when it becomes working and stable
		Args: func(cmd *cobra.Command, args []string) error {
			if repoPath == "" {
				var err error
				repoPath, err = os.Getwd()
				if err != nil {
					return err
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&repoPath, "repo", "", "Path to the repository")

	cmd.AddCommand(
		newRepoInitCmd(env),
		newRepoOwnerCmd(env),
		newRepoCompCmd(env),
		newRepoAddCompCmd(env),
		newRepoYankCompCmd(env),
		newRepoDelCompCmd(env),
		newRepoGenkeyCmd(env),
	)
	return cmd
}

// the `repo add` sub command
func newRepoAddCompCmd(env *meta.Environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <component-id> <platform> <version> <file>",
		Short: "Add a file to a component",
		Long:  `Add a file to a component, and set its metadata of platform ID and version.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 4 {
				return cmd.Help()
			}

			return addCompFile(repoPath, args[0], args[1], args[2], args[3])
		},
	}

	return cmd
}

func addCompFile(repo, id, platform, version, file string) error {
	// TODO
	return nil
}

// the `repo component` sub command
func newRepoCompCmd(env *meta.Environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component <id> <description>",
		Short: "Create a new component in the repository",
		Long:  `Create a new component in the repository, and sign with the local owner key.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}

			return createComp(repoPath, args[0], args[1])
		},
	}

	return cmd
}

func createComp(repo, id, name string) error {
	// TODO
	return nil
}

// the `repo del` sub command
func newRepoDelCompCmd(env *meta.Environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "del <component> [version]",
		Short: "Delete a component from the repository",
		Long: `Delete a component from the repository. If version is not specified, all versions
of the given component will be deleted.
Manifests and files of a deleted component will be removed from the repository,
clients can no longer fetch the component, but files already download by clients
may still be available for them.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			compVer := ""
			switch len(args) {
			case 2:
				compVer = args[1]
			default:
				return cmd.Help()
			}

			return delComp(repoPath, args[0], compVer)
		},
	}

	return cmd
}

func delComp(repo, id, version string) error {
	// TODO
	return nil
}

// the `repo genkey` sub command
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

// the `repo init` sub command
func newRepoInitCmd(env *meta.Environment) *cobra.Command {
	var (
		pubKey  string // public key of root
		privKey string // private key of root
	)
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialise an empty repository",
		Long: `Initialise an empty TiUP repository at given path. If path is not specified, the
current working directory (".") will be used.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				repoPath = args[0]
			}

			// create the target path if not exist
			if utils.IsNotExist(repoPath) {
				var err error
				if err = os.Mkdir(repoPath, 0755); err != nil {
					return err
				}
			}
			// init requires an empty path to use
			empty, err := utils.IsEmptyDir(repoPath)
			if err != nil {
				return err
			}
			if !empty {
				return errors.Errorf("the target path '%s' is not an empty directory", repoPath)
			}

			return initRepo(repoPath)
		},
	}

	cmd.Flags().StringVar(&pubKey, "pubkey", "", "Path to the public key file")
	cmd.Flags().StringVar(&privKey, "privkey", "", "Path to the private key file")

	return cmd
}

func initRepo(path string) error {
	currTime := time.Now().UTC()
	// TODO: set key store

	// initial manifests
	newManifests := make([]repository.ValidManifest, 0)

	// init the root manifest
	root := repository.NewRoot(currTime)
	newManifests = append(newManifests, root)

	// init index
	newManifests = append(newManifests, repository.NewIndex(currTime))

	// snapshot and timestamp are the last two manifests to be initialised
	// init snapshot
	snapshot := repository.NewSnapshot(currTime).SetVersions(newManifests)
	newManifests = append(newManifests, snapshot)

	// init timestamp
	timestamp, err := repository.NewTimestamp(currTime).SetSnapshot(snapshot)
	if err != nil {
		return err
	}
	newManifests = append(newManifests, timestamp)

	// root and snapshot has meta of each other inside themselves, but it's ok here
	// as we are still during the init process, not version bump needed
	root.SetRoles(newManifests)

	return repository.BatchSaveManifests(path, newManifests)
}

// the `repo owner` sub command
func newRepoOwnerCmd(env *meta.Environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "owner <id> <name>",
		Short: "Create a new owner for the repository",
		Long: `Create a new owner role for the repository, the owner can then perform management
actions on authorized resources.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}

			return createOwner(repoPath, args[0], args[1])
		},
	}

	return cmd
}

func createOwner(repo, id, name string) error {
	// TODO
	return nil
}

// the `repo yank` sub command
func newRepoYankCompCmd(env *meta.Environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "yank <component> [version]",
		Short: "Yank a component in the repository",
		Long: `Yank a component in the repository. If version is not specified, all versions
of the given component will be yanked.
A yanked component is still in the repository, but not visible to client, and is
no longer considered stable to use. A yanked component is expected to be removed
from the repository in the future.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			compVer := ""
			switch len(args) {
			case 2:
				compVer = args[1]
			default:
				return cmd.Help()
			}

			return yankComp(repoPath, args[0], compVer)
		},
	}

	return cmd
}

func yankComp(repo, id, version string) error {
	// TODO
	return nil
}
