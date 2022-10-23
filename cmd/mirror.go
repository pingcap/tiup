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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
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
			teleCommand = cmd.CommandPath()
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
		newMirrorShowCmd(),
		newMirrorListCmd(),
		newMirrorSetCmd(),
		newMirrorAddCmd(),
		newMirrorModifyCmd(),
		newMirrorRenewCmd(),
		newMirrorGrantCmd(),
		newMirrorRotateCmd(),
		newTransferOwnerCmd(),
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
		Long:  fmt.Sprintf("Add signatures to a manifest file; if no key file is specified, ~/.tiup/keys/%s will be used", localdata.DefaultPrivateKeyName),
		RunE: func(cmd *cobra.Command, args []string) error {
			teleCommand = cmd.CommandPath()
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
				data, err := client.Get(context.TODO(), args[0])
				if err != nil {
					return err
				}

				if data, err = v1manifest.SignManifestData(data, privKey); err != nil {
					return err
				}

				if _, err = client.Post(context.TODO(), args[0], bytes.NewBuffer(data)); err != nil {
					return err
				}

				return nil
			}

			data, err := os.ReadFile(args[0])
			if err != nil {
				return perrs.Annotatef(err, "open manifest file %s", args[0])
			}

			if data, err = v1manifest.SignManifestData(data, privKey); err != nil {
				return err
			}

			if err = os.WriteFile(args[0], data, 0664); err != nil {
				return perrs.Annotatef(err, "write manifest file %s", args[0])
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&privPath, "key", "k", "", "Specify the private key path")
	cmd.Flags().IntVarP(&timeout, "timeout", "", timeout, "Specify the timeout when access the network")

	return cmd
}

// the `mirror show` sub command
func newMirrorShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show mirror address for single mirror",
		Long:  `Show current mirror address, multi-mirror is not supported`,
		RunE: func(cmd *cobra.Command, args []string) error {
			teleCommand = cmd.CommandPath()
			fmt.Println(environment.Mirror())
			return nil
		},
	}

	return cmd
}

// the `mirror list` sub command
// for multi-mirrors
func newMirrorListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all mirror address",
		Long:  `List all mirror address for multi-mirror`,
		RunE: func(cmd *cobra.Command, args []string) error {
			teleCommand = cmd.CommandPath()

			r := &listResult{
				cmpTable: [][]string{
					{
						"ID", "Name", "Address",
					},
				},
			}

			for i, m := range tiupC.ListMirrors() {
				r.cmpTable = append(r.cmpTable, []string{
					fmt.Sprintf("%d", i+1),
					m.Name,
					m.GetURL(),
				})
			}

			r.print()
			return nil
		},
	}

	return cmd
}

// the `mirror set` sub command
func newMirrorSetCmd() *cobra.Command {
	var (
		root  string
		reset bool
	)
	cmd := &cobra.Command{
		Use:   "set <mirror-addr>",
		Short: "Set mirror address",
		Long: `Set mirror address, the address could be an URL or a path to the repository
directory. Relative paths will not be expanded, so absolute paths are recommended.
The root manifest in $TIUP_HOME will be replaced with the one in given repository automatically.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			teleCommand = cmd.CommandPath()
			if !reset && len(args) != 1 {
				return cmd.Help()
			}
			var addr string
			if reset {
				addr = repository.DefaultMirror
			} else {
				addr = args[0]
			}
			// expand relative path
			if !strings.HasPrefix(addr, "http") {
				var err error
				addr, err = filepath.Abs(addr)
				if err != nil {
					return err
				}
			}

			profile := localdata.InitProfile()
			if err := profile.ResetMirror(addr, root); err != nil {
				log.Errorf("Failed to set mirror: %s\n", err.Error())
				return err
			}
			fmt.Printf("Successfully set mirror to %s\n", addr)
			return nil
		},
	}
	cmd.Flags().StringVarP(&root, "root", "r", root, "Specify the path of `root.json`")
	cmd.Flags().BoolVar(&reset, "reset", false, "Reset mirror to use the default address.")

	return cmd
}

// the `mirror add` sub command
func newMirrorAddCmd() *cobra.Command {
	var (
		root string
		url  string
	)
	cmd := &cobra.Command{
		Use:   "add <mirror-name>",
		Short: "Set mirror name(typically domain)",
		Long:  `Set mirror name, just use domain for mirror with https. For http and local mirror, url param is needed to provide full path.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			teleCommand = cmd.CommandPath()
			if len(args) != 1 {
				return cmd.Help()
			}
			singleMirror := localdata.SingleMirror{Name: args[0], URL: url}

			var rootJSON io.ReadCloser
			var err error
			if root != "" {
				rootJSON, err = os.Open(root)
				if err != nil {
					return err
				}
			} else {
				root = singleMirror.GetURL() + "/root.json"
				fmt.Println(color.YellowString("WARN: adding root certificate via internet: %s", root))
				resp, err := http.Get(root)
				if err != nil {
					return err
				}
				rootJSON = resp.Body
			}
			defer rootJSON.Close()

			err = tiupC.AddMirror(singleMirror, rootJSON)
			if err != nil {
				return err
			}
			fmt.Printf("Successfully add mirror %s (%s)\n", singleMirror.Name, singleMirror.GetURL())
			return tiupC.SaveConfig()
		},
	}
	cmd.Flags().StringVar(&root, "root", root, "Specify the path of `root.json`")
	cmd.Flags().StringVar(&url, "url", root, "Specify the url of mirror")

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
			teleCommand = cmd.CommandPath()
			if len(args) < 1 {
				return cmd.Help()
			}

			id := args[0]
			if name == "" {
				fmt.Printf("No --name is given, using %s as default\n", id)
				name = id
			}

			// the privPath can point to a public key becase the Public method of KeyInfo works on both priv and pub keys
			privKey, err := loadPrivKey(privPath)
			if err != nil {
				return err
			}
			pubKey, err := privKey.Public()
			if err != nil {
				return err
			}
			keyID, err := pubKey.ID()
			if err != nil {
				return err
			}

			env := environment.GlobalEnv()
			err = env.V1Repository().Mirror().Grant(id, name, pubKey)
			if err == nil {
				log.Infof("Granted new owner %s(%s) with public key %s.", id, name, keyID)
			}
			return err
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Specify the name of the owner, default: id of the owner")
	cmd.Flags().StringVarP(&privPath, "key", "k", "", "Specify the path to the private or public key of the owner")

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
			teleCommand = cmd.CommandPath()
			if len(args) != 1 {
				return cmd.Help()
			}

			component := args[0]

			env := environment.GlobalEnv()

			comp, ver := environment.ParseCompVersion(component)
			m, err := env.V1Repository().GetComponentManifest(comp, true)
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
					return perrs.New("nightly version can't be yanked")
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

// the `mirror renew` sub command
func newMirrorRenewCmd() *cobra.Command {
	var privPath string
	var days int

	cmd := &cobra.Command{
		Use:   "renew <component> [flags]",
		Short: "Renew the manifest of a published component.",
		Long:  "Renew the manifest of a published component, bump version and extend its expire time.",
		RunE: func(cmd *cobra.Command, args []string) error {
			teleCommand = cmd.CommandPath()
			if len(args) != 1 {
				return cmd.Help()
			}

			component := args[0]

			env := environment.GlobalEnv()

			comp, _ := environment.ParseCompVersion(component)
			m, err := env.V1Repository().GetComponentManifest(comp, true)
			if err != nil {
				// ignore manifest expiration error
				if !v1manifest.IsExpirationError(perrs.Cause(err)) {
					return err
				}
				fmt.Println(err)
			}

			if days > 0 {
				v1manifest.RenewManifest(m, time.Now(), time.Hour*24*time.Duration(days))
			} else {
				v1manifest.RenewManifest(m, time.Now())
			}

			manifest, err := sign(privPath, m)
			if err != nil {
				return err
			}

			return env.V1Repository().Mirror().Publish(manifest, &model.PublishInfo{})
		},
	}

	cmd.Flags().StringVarP(&privPath, "key", "k", "", "private key path")
	cmd.Flags().IntVar(&days, "days", 0, "after how many days the manifest expires, 0 means builtin default values of manifests")
	return cmd
}

// the `mirror transfer-owner` sub command
func newTransferOwnerCmd() *cobra.Command {
	addr := "0.0.0.0:8080"

	cmd := &cobra.Command{
		Use:   "transfer-owner <component> <new-owner>",
		Short: "Transfer component to another owner",
		Long:  "Transfer component to another owner, this must be done on the server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			teleCommand = cmd.CommandPath()
			if len(args) != 2 {
				return cmd.Help()
			}

			component := args[0]
			newOwnerName := args[1]
			env := environment.GlobalEnv()

			// read current manifests
			index, err := env.V1Repository().FetchIndexManifest()
			if err != nil {
				return err
			}
			newOwner, found := index.Owners[newOwnerName]
			if !found {
				return fmt.Errorf("new owner '%s' is not in the available owner list", newOwnerName)
			}

			m, err := env.V1Repository().GetComponentManifest(component, true)
			if err != nil {
				return err
			}
			v1manifest.RenewManifest(m, time.Now())

			// validate new owner's authorization
			newCompManifest, err := rotate.ServeComponent(addr, &newOwner, m)
			if err != nil {
				return err
			}

			// update owner info
			return env.V1Repository().Mirror().Publish(newCompManifest, &model.PublishInfo{
				Owner: newOwnerName,
			})
		},
	}

	cmd.Flags().StringVarP(&addr, "addr", "", addr, "listen address:port when starting the temp server for signing")

	return cmd
}

// the `mirror rotate` sub command
func newMirrorRotateCmd() *cobra.Command {
	addr := "0.0.0.0:8080"
	keyDir := ""

	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotate root.json",
		Long:  "Rotate root.json make it possible to modify root.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			teleCommand = cmd.CommandPath()

			e, err := environment.InitEnv(repoOpts, repository.MirrorOptions{KeyDir: keyDir})
			if err != nil {
				if errors.Is(perrs.Cause(err), v1manifest.ErrLoadManifest) {
					log.Warnf("Please check for root manifest file, you may download one from the repository mirror, or try `tiup mirror set` to force reset it.")
				}
				return err
			}
			environment.SetGlobalEnv(e)

			root, err := editLatestRootManifest()
			if err != nil {
				return err
			}

			manifest, err := rotate.ServeRoot(addr, root)
			if err != nil {
				return err
			}

			return environment.GlobalEnv().V1Repository().Mirror().Rotate(manifest)
		},
	}
	cmd.Flags().StringVarP(&addr, "addr", "", addr, "listen address:port when starting the temp server for rotating")
	cmd.Flags().StringVarP(&keyDir, "key-dir", "", keyDir, "specify the directory where stores the private keys")

	return cmd
}

func editLatestRootManifest() (*v1manifest.Root, error) {
	root, err := environment.GlobalEnv().V1Repository().FetchRootManifest()
	if err != nil {
		return nil, err
	}

	file, err := os.CreateTemp(os.TempDir(), "*.root.json")
	if err != nil {
		return nil, perrs.Annotate(err, "create temp file for root.json")
	}
	defer file.Close()
	name := file.Name()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")
	if err := encoder.Encode(root); err != nil {
		return nil, perrs.Annotate(err, "encode root.json")
	}
	if err := file.Close(); err != nil {
		return nil, perrs.Annotatef(err, "close %s", name)
	}
	if err := utils.OpenFileInEditor(name); err != nil {
		return nil, err
	}

	root = &v1manifest.Root{}
	file, err = os.Open(name)
	if err != nil {
		return nil, perrs.Annotatef(err, "open %s", name)
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(root); err != nil {
		return nil, perrs.Annotatef(err, "decode %s", name)
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
		return nil, perrs.Annotate(err, "decode key")
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
			teleCommand = cmd.CommandPath()
			if len(args) != 4 {
				return cmd.Help()
			}

			component, version, tarpath, entry := args[0], args[1], args[2], args[3]
			flagSet := set.NewStringSet()
			cmd.Flags().Visit(func(f *pflag.Flag) {
				flagSet.Insert(f.Name)
			})

			if err := validatePlatform(goos, goarch); err != nil {
				return err
			}

			hashes, length, err := ru.HashFile(tarpath)
			if err != nil {
				return err
			}
			fmt.Printf("uploading %s with %d bytes, sha256: %v ...\n",
				tarpath, length, hashes[v1manifest.SHA256])

			tarfile, err := os.Open(tarpath)
			if err != nil {
				return perrs.Annotatef(err, "open tarball: %s", tarpath)
			}
			defer tarfile.Close()

			publishInfo := &model.PublishInfo{
				ComponentData: &model.TarInfo{Reader: tarfile, Name: fmt.Sprintf("%s-%s-%s-%s.tar.gz", component, version, goos, goarch)},
			}

			var reqErr error
			pubErr := utils.Retry(func() error {
				err := doPublish(component, version, entry, desc,
					publishInfo, hashes, length,
					standalone, hidden, privPath,
					goos, goarch, flagSet,
				)
				if err != nil {
					// retry if the error is manifest too old or validation failed
					if err == repository.ErrManifestTooOld ||
						errors.Is(perrs.Cause(err), utils.ErrValidateChecksum) ||
						strings.Contains(err.Error(), "INVALID TARBALL") {
						fmt.Printf("server returned an error: %s, retry...\n", err)
						if _, ferr := tarfile.Seek(0, 0); ferr != nil { // reset the reader
							return ferr
						}
						return err // return err to trigger next retry
					}
					reqErr = err // keep the error info
				}
				return nil // return nil to end the retry loop
			}, utils.RetryOption{
				Attempts: 10,
				Delay:    time.Second * 2,
				Timeout:  time.Minute * 10,
			})
			if reqErr != nil {
				return reqErr
			}
			return pubErr
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

func doPublish(
	component, version, entry, desc string,
	publishInfo *model.PublishInfo,
	hashes map[string]string, length int64,
	standalone, hidden bool,
	privPath, goos, goarch string,
	flagSet set.StringSet,
) error {
	env := environment.GlobalEnv()
	env.V1Repository().PurgeTimestamp()
	m, err := env.V1Repository().GetComponentManifest(component, true)
	if err != nil {
		if perrs.Cause(err) != repository.ErrUnknownComponent {
			return err
		}
		fmt.Printf("Creating component %s\n", component)
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
		return perrs.Errorf("platform %s/%s not supported", goos, goarch)
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
			teleCommand = cmd.CommandPath()
			env := environment.GlobalEnv()
			privPath := env.Profile().Path(localdata.KeyInfoParentDir, name+".json")
			keyDir := filepath.Dir(privPath)
			if utils.IsNotExist(keyDir) {
				if err := os.Mkdir(keyDir, 0755); err != nil {
					return perrs.Annotate(err, "create private key dir")
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
					log.Warnf("Warning: private key already exists (%s), skipped", privPath)
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

				fmt.Printf("Private key has been written to %s\n", privPath)
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

	cmd.Flags().BoolVarP(&showPublic, "public", "p", showPublic, "Show public content")
	cmd.Flags().BoolVar(&saveKey, "save", false, "Save public key to a file in the current working dir")
	cmd.Flags().StringVarP(&name, "name", "n", "private", "The file name of the key")

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
		Long: `Initialize an empty TiUP repository at given path.
The specified path must be an empty directory.
If the path does not exist, a new directory will be created.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			teleCommand = cmd.CommandPath()
			if len(args) != 1 {
				return cmd.Help()
			}
			repoPath := args[0]

			// create the target path if not exist
			if utils.IsNotExist(repoPath) {
				var err error
				log.Infof("Target path \"%s\" does not exist, creating new directory...", repoPath)
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
				return perrs.Errorf("the target path '%s' is not an empty directory", repoPath)
			}

			if keyDir == "" {
				keyDir = path.Join(repoPath, "keys")
			}
			return initRepo(repoPath, keyDir)
		},
	}

	cmd.Flags().StringVarP(&keyDir, "key-dir", "k", "", "Path to write the private key files")

	return cmd
}

func initRepo(path, keyDir string) error {
	log.Infof("Initializing empty new repository at \"%s\", private keys will be stored in \"%s\"...", path, keyDir)
	err := v1manifest.Init(path, keyDir, time.Now().UTC())
	if err != nil {
		log.Errorf("Initializing new repository failed.")
		return err
	}
	log.Infof("New repository initialized at \"%s\", private keys are stored in \"%s\".", path, keyDir)
	log.Infof("Use `%s` command to set and use the new repository.", color.CyanString("tiup mirror set %s", path))
	return nil
}

// the `mirror merge` sub command
func newMirrorMergeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "merge <mirror-dir-1> [mirror-dir-N]",
		Example: `	tiup mirror merge tidb-community-v4.0.1					# merge v4.0.1 into current mirror
	tiup mirror merge tidb-community-v4.0.1 tidb-community-v4.0.2		# merge v4.0.1 and v4.0.2 into current mirror`,
		Short: "Merge two or more offline mirror",
		RunE: func(cmd *cobra.Command, args []string) error {
			teleCommand = cmd.CommandPath()
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
			for name, comp := range index.Components {
				if comp.Yanked {
					continue
				}
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
		Example: `  tiup mirror clone /path/to/local --arch amd64,arm64 --os linux,darwin    # Specify the architectures and OSs
  tiup mirror clone /path/to/local --os linux v6.1.0 v5.4.0              # Specify multiple versions 
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
			teleCommand = cmd.CommandPath()
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
				return perrs.New("component list doesn't contain components")
			}

			if err = repo.Mirror().Open(); err != nil {
				return err
			}
			defer func() {
				err = repo.Mirror().Close()
				if err != nil {
					log.Errorf("Failed to close mirror: %s\n", err.Error())
				}
			}()

			var versionMapper = func(comp string) string {
				return spec.TiDBComponentVersion(comp, "")
			}

			// format input versions
			versionList := make([]string, 0)
			for _, ver := range args[1:] {
				v, err := utils.FmtVer(ver)
				if err != nil {
					return err
				}
				versionList = append(versionList, v)
			}

			return repository.CloneMirror(repo, components, versionMapper, args[0], versionList, options)
		},
	}

	cmd.Flags().SortFlags = false
	cmd.Flags().BoolVarP(&options.Full, "full", "f", false, "Build a full mirrors repository")
	cmd.Flags().StringSliceVarP(&options.Archs, "arch", "a", []string{"amd64", "arm64"}, "Specify the downloading architecture")
	cmd.Flags().StringSliceVarP(&options.OSs, "os", "o", []string{"linux", "darwin"}, "Specify the downloading os")
	cmd.Flags().BoolVarP(&options.Prefix, "prefix", "", false, "Download the version with matching prefix")
	cmd.Flags().UintVarP(&options.Jobs, "jobs", "", 1, "Specify the number of concurrent download jobs")

	originHelpFunc := cmd.HelpFunc()
	cmd.SetHelpFunc(func(command *cobra.Command, args []string) {
		if !initialized {
			_ = initMirrorCloneExtraArgs(command)
		}
		originHelpFunc(command, args)
	})

	return cmd
}
