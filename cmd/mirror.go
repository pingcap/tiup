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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/repository/model"
	ru "github.com/pingcap/tiup/pkg/repository/utils"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/server/rotate"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return nil
		},
	}

	cmd.AddCommand(
		newMirrorInitCmd(),
		newMirrorSignCmd(),
		newMirrorGenkeyCmd(),
		newMirrorCloneCmd(),
		newMirrorMergeCmd(),
		newMirrorPublishCmd(),
		newMirrorSetCmd(),
		newMirrorModifyCmd(),
		newMirrorGrantCmd(),
		newMirrorRotateCmd(),
	)

	return cmd
}

// the `mirror sign` sub command
func newMirrorSignCmd() *cobra.Command {
	privPath := ""
	timeout := 10

	cmd := &cobra.Command{
		Use:   "sign <manifest-file>",
		Short: "Add signatures to a manifest file",
		Long:  fmt.Sprintf("Add signatures to a manifest file, if no key file specified, the ~/.tiup/keys/%s will be used", localdata.DefaultPrivateKeyName),
		RunE: func(cmd *cobra.Command, args []string) error {
			env := environment.GlobalEnv()
			if len(args) < 1 {
				return cmd.Help()
			}

			if privPath == "" {
				privPath = env.Profile().Path(localdata.KeyInfoParentDir, localdata.DefaultPrivateKeyName)
			}

			privKey, err := loadPrivKey(privPath)
			if err != nil {
				return err
			}

			if strings.HasPrefix(args[0], "http") {
				client := utils.NewHTTPClient(time.Duration(timeout)*time.Second, nil)
				data, err := client.Get(args[0])
				if err != nil {
					return err
				}

				if data, err = v1manifest.SignManifestData(data, privKey); err != nil {
					return err
				}

				if _, err = client.Post(args[0], bytes.NewBuffer(data)); err != nil {
					return err
				}

				return nil
			}

			data, err := os.ReadFile(args[0])
			if err != nil {
				return errors.Annotatef(err, "open manifest file %s", args[0])
			}

			if data, err = v1manifest.SignManifestData(data, privKey); err != nil {
				return err
			}

			if err = os.WriteFile(args[0], data, 0664); err != nil {
				return errors.Annotatef(err, "write manifest file %s", args[0])
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&privPath, "key", "k", "", "Specify the private key path")
	cmd.Flags().IntVarP(&timeout, "timeout", "", timeout, "Specify the timeout when access the network")

	return cmd
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
				log.Errorf("Failed to set mirror: %s\n", err.Error())
				return err
			}
			fmt.Printf("Set mirror to %s success\n", addr)
			return nil
		},
	}
	cmd.Flags().StringVarP(&root, "root", "r", root, "Specify the path of `root.json`")
	return cmd
}

// the `mirror grant` sub command
func newMirrorGrantCmd() *cobra.Command {
	name := ""
	privPath := ""

	cmd := &cobra.Command{
		Use:   "grant <id>",
		Short: "grant a new owner",
		Long:  "grant a new owner to current mirror",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}

			id := args[0]
			if name == "" {
				fmt.Printf("The --name is not specified, using %s as default\n", id)
				name = id
			}

			// the privPath can point to a public key becase the Public method of KeyInfo works on both priv and pub key
			privKey, err := loadPrivKey(privPath)
			if err != nil {
				return err
			}
			pubKey, err := privKey.Public()
			if err != nil {
				return err
			}

			env := environment.GlobalEnv()
			return env.V1Repository().Mirror().Grant(id, name, pubKey)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Specify the name of the owner, default: id of the owner")
	cmd.Flags().StringVarP(&privPath, "key", "k", "", "Specify the private key path of the owner")

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
		Use:   "modify <component>[:version] [flags]",
		Short: "Modify published component",
		Long:  "Modify component attributes (hidden, standalone, yanked)",
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
				if ver.IsNightly() {
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

// the `mirror rotate` sub command
func newMirrorRotateCmd() *cobra.Command {
	addr := "0.0.0.0:8080"

	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotate root.json",
		Long:  "Rotate root.json make it possible to modify root.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := editLatestRootManifest()
			if err != nil {
				return err
			}

			manifest, err := rotate.Serve(addr, root)
			if err != nil {
				return err
			}

			return environment.GlobalEnv().V1Repository().Mirror().Rotate(manifest)
		},
	}
	cmd.Flags().StringVarP(&addr, "addr", "", addr, "listen address:port when starting the temp server for rotating")

	return cmd
}

func editLatestRootManifest() (*v1manifest.Root, error) {
	root, err := environment.GlobalEnv().V1Repository().FetchRootManfiest()
	if err != nil {
		return nil, err
	}

	file, err := os.CreateTemp(os.TempDir(), "*.root.json")
	if err != nil {
		return nil, errors.Annotate(err, "create temp file for root.json")
	}
	defer file.Close()
	name := file.Name()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")
	if err := encoder.Encode(root); err != nil {
		return nil, errors.Annotate(err, "encode root.json")
	}
	if err := file.Close(); err != nil {
		return nil, errors.Annotatef(err, "close %s", name)
	}
	if err := utils.OpenFileInEditor(name); err != nil {
		return nil, err
	}

	root = &v1manifest.Root{}
	file, err = os.Open(name)
	if err != nil {
		return nil, errors.Annotatef(err, "open %s", name)
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(root); err != nil {
		return nil, errors.Annotatef(err, "decode %s", name)
	}

	return root, nil
}

func loadPrivKey(privPath string) (*v1manifest.KeyInfo, error) {
	env := environment.GlobalEnv()
	if privPath == "" {
		privPath = env.Profile().Path(localdata.KeyInfoParentDir, localdata.DefaultPrivateKeyName)
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

func loadPrivKeys(keysDir string) (map[string]*v1manifest.KeyInfo, error) {
	keys := map[string]*v1manifest.KeyInfo{}

	err := filepath.Walk(keysDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ki, err := loadPrivKey(path)
		if err != nil {
			return err
		}

		id, err := ki.ID()
		if err != nil {
			return err
		}

		keys[id] = ki
		return nil
	})

	if err != nil {
		return nil, err
	}

	return keys, nil
}

func sign(privPath string, signed v1manifest.ValidManifest) (*v1manifest.Manifest, error) {
	ki, err := loadPrivKey(privPath)
	if err != nil {
		return nil, err
	}

	return v1manifest.SignManifest(signed, ki)
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

			tarfile, err := os.Open(tarpath)
			if err != nil {
				return errors.Annotatef(err, "open tarball: %s", tarpath)
			}
			defer tarfile.Close()

			publishInfo := &model.PublishInfo{
				ComponentData: &model.TarInfo{Reader: tarfile, Name: fmt.Sprintf("%s-%s-%s-%s.tar.gz", component, version, goos, goarch)},
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
				fmt.Println("This is not a new component, --standalone and --hide flag will be omitted")
			}

			m = repository.UpdateManifestForPublish(m, component, version, entry, goos, goarch, desc, v1manifest.FileHash{
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
	case "linux/amd64",
		"linux/arm64",
		"darwin/amd64",
		"darwin/arm64":
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
		name       string
	)

	cmd := &cobra.Command{
		Use:   "genkey",
		Short: "Generate a new key pair",
		Long:  `Generate a new key pair that can be used to sign components.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			env := environment.GlobalEnv()
			privPath := env.Profile().Path(localdata.KeyInfoParentDir, name+".json")
			keyDir := filepath.Dir(privPath)
			if utils.IsNotExist(keyDir) {
				if err := os.Mkdir(keyDir, 0755); err != nil {
					return errors.Annotate(err, "create private key dir")
				}
			}

			var ki *v1manifest.KeyInfo
			var err error
			if showPublic {
				ki, err = loadPrivKey(privPath)
				if err != nil {
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
			} else {
				if utils.IsExist(privPath) {
					log.Warnf("Warning: private key already exists(%s), skip", privPath)
					return nil
				}

				ki, err = v1manifest.GenKeyInfo()
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

				if err := json.NewEncoder(f).Encode(ki); err != nil {
					return err
				}

				fmt.Printf("Private key has been writeen to %s\n", privPath)
			}

			if saveKey {
				pubKey, err := ki.Public()
				if err != nil {
					return err
				}
				pubPath, err := v1manifest.SaveKeyInfo(pubKey, "public", "")
				if err != nil {
					return err
				}
				fmt.Printf("Public key has been written to current working dir: %s\n", pubPath)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&showPublic, "public", "p", showPublic, "show public content")
	cmd.Flags().BoolVar(&saveKey, "save", false, "Save public key to a file at current working dir")
	cmd.Flags().StringVarP(&name, "name", "n", "private", "the file name of the key")

	return cmd
}

// the `mirror init` sub command
func newMirrorInitCmd() *cobra.Command {
	var (
		keyDir string // Directory to write genreated key files
	)
	cmd := &cobra.Command{
		Use:   "init <path>",
		Short: "Initialize an empty repository",
		Long: `Initialize an empty TiUP repository at given path. If path is not specified, the
current working directory (".") will be used.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}
			repoPath := args[0]

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

			if keyDir == "" {
				keyDir = path.Join(repoPath, "keys")
			}
			return initRepo(repoPath, keyDir)
		},
	}

	cmd.Flags().StringVarP(&keyDir, "key-dir", "k", "", "Path to write the private key file")

	return cmd
}

func initRepo(path, keyDir string) error {
	return v1manifest.Init(path, keyDir, time.Now().UTC())
}

// the `mirror merge` sub command
func newMirrorMergeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "merge <mirror-dir-1> [mirror-dir-N]",
		Example: `	tiup mirror merge tidb-community-v4.0.1					# merge v4.0.1 into current mirror
	tiup mirror merge tidb-community-v4.0.1 tidb-community-v4.0.2		# merge v4.0.1 and v4.0.2 into current mirror`,
		Short: "Merge two or more offline mirror",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}

			sources := args

			env := environment.GlobalEnv()
			baseMirror := env.V1Repository().Mirror()

			sourceMirrors := []repository.Mirror{}
			for _, source := range sources {
				sourceMirror := repository.NewMirror(source, repository.MirrorOptions{})
				if err := sourceMirror.Open(); err != nil {
					return err
				}
				defer sourceMirror.Close()

				sourceMirrors = append(sourceMirrors, sourceMirror)
			}

			keys, err := loadPrivKeys(env.Profile().Path(localdata.KeyInfoParentDir))
			if err != nil {
				return err
			}

			return repository.MergeMirror(keys, baseMirror, sourceMirrors...)
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

			var versionMapper = func(comp string) string {
				return spec.TiDBComponentVersion(comp, "")
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
