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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/repository/model"
	ru "github.com/pingcap/tiup/pkg/repository/utils"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	repoPath string
)

func newMirrorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mirror <command>",
		Short: "Manage a repository mirror for TiUP components",
		Long: `The 'mirror' command is used to manage a component repository for TiUP, you can use
it to create a private repository, or to add new component to an existing repository.
The repository can be used either online or offline.
It also provides some useful utilities to help managing keys, users and versions
of components or the repository itself.`,
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
		newMirrorInitCmd(),
		newMirrorSignCmd(),
		newMirrorOwnerCmd(),
		newMirrorCompCmd(),
		newMirrorAddCompCmd(),
		newMirrorYankCompCmd(),
		newMirrorDelCompCmd(),
		newMirrorGenkeyCmd(),
		newMirrorCloneCmd(),
		newMirrorMergeCmd(),
		newMirrorPublishCmd(),
		newMirrorSetCmd(),
		newMirrorModifyCmd(),
	)

	return cmd
}

// the `mirror sign` sub command
func newMirrorSignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sign <manifest-file> [key-files]",
		Short: "Add signatures to a manifest file",
		Long:  "Add signatures to a manifest file, if no key file specified, the ~/.tiup/keys/private.json will be used",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := environment.GlobalEnv()
			if len(args) < 1 {
				return cmd.Help()
			}

			if len(args) == 1 {
				return v1manifest.SignManifestFile(args[0], env.Profile().Path(localdata.KeyInfoParentDir, "private.json"))
			}
			return v1manifest.SignManifestFile(args[0], args[1:]...)
		},
	}

	return cmd
}

// the `mirror add` sub command
func newMirrorAddCompCmd() *cobra.Command {
	var nightly bool // if this is a nightly version
	cmd := &cobra.Command{
		Use:    "add <component-id> <platform> <version> <file>",
		Short:  "Add a file to a component",
		Long:   `Add a file to a component, and set its metadata of platform ID and version.`,
		Hidden: true, // WIP, remove when it becomes working and stable
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 4 {
				return cmd.Help()
			}

			return addCompFile(repoPath, args[0], args[1], args[2], args[3], nightly)
		},
	}

	// If adding legacy nightly build (e.g., add a version from yesterday), just
	// omit the flag to treat it as normal versions
	cmd.Flags().BoolVar(&nightly, "nightly", false, "Mark this version as the latest nightly build")

	return cmd
}

func addCompFile(repo, id, platform, version, file string, nightly bool) error {
	// TODO
	return nil
}

// the `mirror component` sub command
func newMirrorCompCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "component <id> <description>",
		Short:  "Create a new component in the repository",
		Long:   `Create a new component in the repository, and sign with the local owner key.`,
		Hidden: true, // WIP, remove when it becomes working and stable
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

// the `mirror del` sub command
func newMirrorDelCompCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "del <component> [version]",
		Short: "Delete a component from the repository",
		Long: `Delete a component from the repository. If version is not specified, all versions
of the given component will be deleted.
Manifests and files of a deleted component will be removed from the repository,
clients can no longer fetch the component, but files already download by clients
may still be available for them.`,
		Hidden: true, // WIP, remove when it becomes working and stable
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
	// TODO: implement the func

	// TODO: check if version is the latest nightly, refuse if it is
	return nil
}

// the `mirror set` sub command
func newMirrorSetCmd() *cobra.Command {
	root := ""
	cmd := &cobra.Command{
		Use:   "set <mirror-addr>",
		Short: "set mirror address",
		Long:  "set mirror address, will replace the root certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			addr := args[0]
			profile := environment.GlobalEnv().Profile()
			if err := profile.ResetMirror(addr, root); err != nil {
				fmt.Printf("Failed to set mirror: %s\n", err.Error())
				return err
			}
			fmt.Printf("Set mirror to %s success\n", addr)
			return nil
		},
	}
	cmd.Flags().StringVarP(&root, "root", "r", root, "Specify the path of `root.json`")
	return cmd
}

// the `mirror modify` sub command
func newMirrorModifyCmd() *cobra.Command {
	var privPath string
	desc := ""
	standalone := false
	hidden := false
	yanked := false

	cmd := &cobra.Command{
		Use:  "modify <component>[:version] [flags]",
		Long: "modify component attributes (hidden, standalone, yanked)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			component := args[0]

			env := environment.GlobalEnv()

			comp, ver := environment.ParseCompVersion(component)
			m, err := env.V1Repository().FetchComponentManifest(comp, true)
			if err != nil {
				return err
			}

			v1manifest.RenewManifest(m, time.Now())
			m.Version++
			if desc != "" {
				m.Description = desc
			}

			flagSet := set.NewStringSet()
			cmd.Flags().Visit(func(f *pflag.Flag) {
				flagSet.Insert(f.Name)
			})

			publishInfo := &model.PublishInfo{}
			if ver == "" {
				if flagSet.Exist("standalone") {
					publishInfo.Stand = &standalone
				}
				if flagSet.Exist("hide") {
					publishInfo.Hide = &hidden
				}
				if flagSet.Exist("yank") {
					publishInfo.Yank = &yanked
				}
			} else if flagSet.Exist("yank") {
				if v0manifest.Version(ver).IsNightly() {
					return errors.New("nightly version can't be yanked")
				}
				for p := range m.Platforms {
					vi, ok := m.Platforms[p][ver.String()]
					if !ok {
						continue
					}
					vi.Yanked = yanked
					m.Platforms[p][ver.String()] = vi
				}
			}

			manifest, err := sign(privPath, m)
			if err != nil {
				return err
			}

			return env.V1Repository().Mirror().Publish(manifest, publishInfo)
		},
	}

	cmd.Flags().StringVarP(&privPath, "key", "k", "", "private key path")
	cmd.Flags().StringVarP(&desc, "desc", "", desc, "description of the component")
	cmd.Flags().BoolVarP(&standalone, "standalone", "", standalone, "can this component run directly")
	cmd.Flags().BoolVarP(&hidden, "hide", "", hidden, "is this component visible in list")
	cmd.Flags().BoolVarP(&yanked, "yank", "", yanked, "is this component deprecated")
	return cmd
}

func loadPrivKey(privPath string) (*v1manifest.KeyInfo, error) {
	env := environment.GlobalEnv()
	if privPath == "" {
		privPath = env.Profile().Path(localdata.KeyInfoParentDir, "private.json")
	}

	// Get the private key
	f, err := os.Open(privPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ki := v1manifest.KeyInfo{}
	if err := json.NewDecoder(f).Decode(&ki); err != nil {
		return nil, errors.Annotate(err, "decode key")
	}

	return &ki, nil
}

func sign(privPath string, m v1manifest.ValidManifest) (*v1manifest.Manifest, error) {
	ki, err := loadPrivKey(privPath)
	if err != nil {
		return nil, err
	}

	kid, err := ki.ID()
	if err != nil {
		return nil, err
	}

	sig, err := ki.SignManifest(m)
	if err != nil {
		return nil, err
	}

	manifest := &v1manifest.Manifest{
		Signatures: []v1manifest.Signature{
			{
				KeyID: kid,
				Sig:   sig,
			},
		},
		Signed: m,
	}

	return manifest, nil
}

func updateManifestForPublish(m *v1manifest.Component,
	name, ver, entry, os, arch, desc string,
	filehash v1manifest.FileHash) *v1manifest.Component {
	initTime := time.Now()

	// update manifest
	if m == nil {
		m = v1manifest.NewComponent(name, desc, initTime)
	} else {
		v1manifest.RenewManifest(m, initTime)
		m.Version++
		if desc != "" {
			m.Description = desc
		}
	}

	if strings.Contains(ver, version.NightlyVersion) {
		m.Nightly = ver
	}

	// Remove history nightly
	for plat := range m.Platforms {
		for v := range m.Platforms[plat] {
			if strings.Contains(v, version.NightlyVersion) && v != m.Nightly {
				delete(m.Platforms[plat], v)
			}
		}
	}

	platformStr := fmt.Sprintf("%s/%s", os, arch)
	if m.Platforms[platformStr] == nil {
		m.Platforms[platformStr] = map[string]v1manifest.VersionItem{}
	}

	m.Platforms[platformStr][ver] = v1manifest.VersionItem{
		Entry:    entry,
		Released: initTime.Format(time.RFC3339),
		URL:      fmt.Sprintf("/%s-%s-%s-%s.tar.gz", name, ver, os, arch),
		FileHash: filehash,
	}

	return m
}

// the `mirror publish` sub command
func newMirrorPublishCmd() *cobra.Command {
	var privPath string
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	desc := ""
	standalone := false
	hidden := false

	cmd := &cobra.Command{
		Use:   "publish <comp-name> <version> <tarball> <entry>",
		Short: "Publish a component",
		Long:  "Publish a component to the repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 4 {
				return cmd.Help()
			}

			component, version, tarpath, entry := args[0], args[1], args[2], args[3]

			if err := validatePlatform(goos, goarch); err != nil {
				return err
			}

			hashes, length, err := ru.HashFile(tarpath)
			if err != nil {
				return err
			}

			tarfile, err := os.Open(args[2])
			if err != nil {
				return errors.Annotate(err, "open tarbal")
			}
			defer tarfile.Close()

			publishInfo := &model.PublishInfo{
				ComponentData: &model.TarInfo{tarfile, fmt.Sprintf("%s-%s-%s-%s.tar.gz", component, version, goos, goarch)},
			}

			flagSet := set.NewStringSet()
			cmd.Flags().Visit(func(f *pflag.Flag) {
				flagSet.Insert(f.Name)
			})
			env := environment.GlobalEnv()
			m, err := env.V1Repository().FetchComponentManifest(component, true)
			if err != nil {
				fmt.Printf("Fetch local manifest: %s\n", err.Error())
				fmt.Printf("Failed to load component manifest, create a new one\n")
				publishInfo.Stand = &standalone
				publishInfo.Hide = &hidden
			} else if flagSet.Exist("standalone") || flagSet.Exist("hide") {
				fmt.Println("This is not a new component, --standalone and --hide flag will be omited")
			}

			m = updateManifestForPublish(m, component, version, entry, goos, goarch, desc, v1manifest.FileHash{
				Hashes: hashes,
				Length: uint(length),
			})

			manifest, err := sign(privPath, m)
			if err != nil {
				return err
			}

			return env.V1Repository().Mirror().Publish(manifest, publishInfo)
		},
	}

	cmd.Flags().StringVarP(&privPath, "key", "k", "", "private key path")
	cmd.Flags().StringVarP(&goos, "os", "", goos, "the target operation system")
	cmd.Flags().StringVarP(&goarch, "arch", "", goarch, "the target system architecture")
	cmd.Flags().StringVarP(&desc, "desc", "", desc, "description of the component")
	cmd.Flags().BoolVarP(&standalone, "standalone", "", standalone, "can this component run directly")
	cmd.Flags().BoolVarP(&hidden, "hide", "", hidden, "is this component invisible on listing")
	return cmd
}

func validatePlatform(goos, goarch string) error {
	// Only support any/any, don't support linux/any, any/amd64 .etc.
	if goos == "any" && goarch == "any" {
		return nil
	}

	switch goos + "/" + goarch {
	case "linux/amd64", "linux/arm64", "darwin/amd64":
		return nil
	default:
		return errors.Errorf("platform %s/%s not supported", goos, goarch)
	}
}

// the `mirror genkey` sub command
func newMirrorGenkeyCmd() *cobra.Command {
	var (
		showPublic bool
		saveKey    bool
		privPath   string
	)

	cmd := &cobra.Command{
		Use:   "genkey",
		Short: "Generate a new key pair",
		Long:  `Generate a new key pair that can be used to sign components.`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			env := environment.GlobalEnv()
			privPath = env.Profile().Path(localdata.KeyInfoParentDir, "private.json")
			keyDir := filepath.Dir(privPath)
			if utils.IsNotExist(keyDir) {
				return os.Mkdir(keyDir, 0755)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if showPublic {
				f, err := os.Open(privPath)
				if err != nil {
					return err
				}
				defer f.Close()

				ki := v1manifest.KeyInfo{}
				if err := json.NewDecoder(f).Decode(&ki); err != nil {
					return err
				}
				pki, err := ki.Public()
				if err != nil {
					return err
				}
				id, err := pki.ID()
				if err != nil {
					return err
				}
				content, err := json.MarshalIndent(pki, "", "\t")
				if err != nil {
					return err
				}

				fmt.Printf("KeyID: %s\nKeyContent: \n%s\n", id, string(content))

				// TODO: suggest key type from input, there will also be owner keys
				if saveKey {
					pubKey, err := ki.Public()
					if err != nil {
						return err
					}
					if err = v1manifest.SaveKeyInfo(pubKey, "root", ""); err != nil {
						return err
					}
					fmt.Printf("public key have been write to current working dir\n")
				}
				return nil
			}

			if utils.IsExist(privPath) {
				fmt.Println("Key already exists, skipped")
				return nil
			}

			key, err := v1manifest.GenKeyInfo()
			if err != nil {
				return err
			}

			f, err := os.Create(privPath)
			if err != nil {
				return err
			}
			defer f.Close()

			// set private key permission
			if err = f.Chmod(0600); err != nil {
				return err
			}

			if err := json.NewEncoder(f).Encode(key); err != nil {
				return err
			}

			fmt.Printf("private key have been write to %s\n", privPath)

			// TODO: suggest key type from input, there will also be owner keys
			if saveKey {
				pubKey, err := key.Public()
				if err != nil {
					return err
				}
				if err = v1manifest.SaveKeyInfo(pubKey, "root", ""); err != nil {
					return err
				}
				fmt.Printf("public key have been write to current working dir\n")
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&showPublic, "public", "p", showPublic, fmt.Sprintf("show public content of %s", privPath))
	cmd.Flags().BoolVar(&saveKey, "save", false, "Save public key to a file at current working dir")

	return cmd
}

// the `mirror init` sub command
func newMirrorInitCmd() *cobra.Command {
	var (
		keyDir string // Directory to write genreated key files
	)
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize an empty repository",
		Long: `Initialize an empty TiUP repository at given path. If path is not specified, the
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

			return initRepo(repoPath, keyDir)
		},
	}

	cmd.Flags().StringVarP(&keyDir, "", "i", "", "Path to write the private key file")

	return cmd
}

func initRepo(path, keyDir string) error {
	return v1manifest.Init(path, keyDir, time.Now().UTC())
}

// the `mirror owner` sub command
func newMirrorOwnerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "owner <id> <name>",
		Short: "Create a new owner for the repository",
		Long: `Create a new owner role for the repository, the owner can then perform management
actions on authorized resources.`,
		Hidden: true, // WIP, remove when it becomes working and stable
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

// the `mirror yank` sub command
func newMirrorYankCompCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "yank <component> [version]",
		Short: "Yank a component in the repository",
		Long: `Yank a component in the repository. If version is not specified, all versions
of the given component will be yanked.
A yanked component is still in the repository, but not visible to client, and is
no longer considered stable to use. A yanked component is expected to be removed
from the repository in the future.`,
		Hidden: true, // WIP, remove when it becomes working and stable
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

// the `mirror merge` sub command
func newMirrorMergeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "merge <base> <mirror-dir-1> [mirror-dir-N]",
		Example: `	tiup mirror merge tidb-community-v4.0.0 tidb-community-v4.0.1					# merge v4.0.1 into v4.0.0
	tiup mirror merge tidb-community-v4.0.0 tidb-community-v4.0.1 tidb-community-v4.0.2		# merge v4.0.1 and v4.0.2 into v4.0.0`,
		Short: "Merge two or more offline mirror",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}

			base := args[0]
			sources := args[1:]

			return repository.MergeMirror(base, sources)
		},
	}

	return cmd
}

// the `mirror clone` sub command
func newMirrorCloneCmd() *cobra.Command {
	var (
		options     = repository.CloneOptions{Components: map[string]*[]string{}}
		components  []string
		repo        *repository.V1Repository
		initialized bool
	)

	initMirrorCloneExtraArgs := func(cmd *cobra.Command) error {
		initialized = true
		env := environment.GlobalEnv()
		repo = env.V1Repository()
		index, err := repo.FetchIndexManifest()
		if err != nil {
			return err
		}

		if index != nil && len(index.Components) > 0 {
			for name := range index.Components {
				components = append(components, name)
			}
		}
		sort.Strings(components)

		for _, name := range components {
			options.Components[name] = new([]string)
			cmd.Flags().StringSliceVar(options.Components[name], name, nil, "Specify the versions for component "+name)
		}
		return nil
	}
	cmd := &cobra.Command{
		Use: "clone <target-dir> [global version]",
		Example: `  tiup mirror clone /path/to/local --arch amd64,arm --os linux,darwin    # Specify the architectures and OSs
  tiup mirror clone /path/to/local --full                                # Build a full local mirror
  tiup mirror clone /path/to/local --tikv v4  --prefix                   # Specify the version via prefix
  tiup mirror clone /path/to/local --tidb all --pd all                   # Download all version for specific component`,
		Short:              "Clone a local mirror from remote mirror and download all selected components",
		SilenceUsage:       true,
		DisableFlagParsing: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return initMirrorCloneExtraArgs(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.DisableFlagParsing = false
			err := cmd.ParseFlags(args)
			if err != nil {
				return err
			}
			args = cmd.Flags().Args()
			printHelp, _ := cmd.Flags().GetBool("help")

			if printHelp || len(args) < 1 {
				return cmd.Help()
			}

			if len(components) < 1 {
				return errors.New("component list doesn't contain components")
			}

			if err = repo.Mirror().Open(); err != nil {
				return err
			}
			defer repo.Mirror().Close()

			var versionMapper = func(ver string) string {
				return spec.TiDBComponentVersion(ver, "")
			}

			return repository.CloneMirror(repo, components, versionMapper, args[0], args[1:], options)
		},
	}

	cmd.Flags().SortFlags = false
	cmd.Flags().BoolVarP(&options.Full, "full", "f", false, "Build a full mirrors repository")
	cmd.Flags().StringSliceVarP(&options.Archs, "arch", "a", []string{"amd64", "arm64"}, "Specify the downloading architecture")
	cmd.Flags().StringSliceVarP(&options.OSs, "os", "o", []string{"linux", "darwin"}, "Specify the downloading os")
	cmd.Flags().BoolVarP(&options.Prefix, "prefix", "", false, "Download the version with matching prefix")

	originHelpFunc := cmd.HelpFunc()
	cmd.SetHelpFunc(func(command *cobra.Command, args []string) {
		if !initialized {
			_ = initMirrorCloneExtraArgs(command)
		}
		originHelpFunc(command, args)
	})

	return cmd
}
