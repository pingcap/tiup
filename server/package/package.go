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

package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	pkgver "github.com/pingcap/tiup/pkg/repository/version"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"github.com/spf13/cobra"
)

var mirror = environment.Mirror()
var errNotFound = fmt.Errorf("resource not found")

func main() {
	if err := execute(); err != nil {
		fmt.Println("Packaging component failed:", err)
		os.Exit(1)
	}
}

type packageOptions struct {
	goos       string
	goarch     string
	dir        string
	name       string
	version    string
	entry      string
	desc       string
	standalone bool
	hide       bool
}

func execute() error {
	options := packageOptions{}

	rootCmd := &cobra.Command{
		Use:          "tiup package target",
		Short:        "Package a tiup component and generate package directory",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := chwd(); err != nil {
				return err
			}
			if len(args) < 1 {
				return cmd.Help()
			}

			return pack(args, options)
		},
	}

	rootCmd.Flags().StringVar(&options.goos, "os", runtime.GOOS, "Target OS of the package")
	rootCmd.Flags().StringVar(&options.goarch, "arch", runtime.GOARCH, "Target ARCH of the package")
	rootCmd.Flags().StringVarP(&options.dir, "", "C", "", "Change directory before compress")
	rootCmd.Flags().StringVar(&options.name, "name", "", "Name of the package (required)")
	rootCmd.Flags().StringVar(&options.version, "release", "", "Version of the package (required)")
	rootCmd.Flags().StringVar(&options.entry, "entry", "", "(deprecated) Entry point of the package")
	rootCmd.Flags().StringVar(&options.desc, "desc", "", "(deprecated) Description of the package")
	rootCmd.Flags().BoolVar(&options.standalone, "standalone", false, "(deprecated) Can the component run standalone")
	rootCmd.Flags().BoolVar(&options.hide, "hide", false, "(deprecated) Don't show the component in `tiup list`")

	_ = rootCmd.MarkFlagRequired("name")
	_ = rootCmd.MarkFlagRequired("release")
	_ = rootCmd.MarkFlagRequired("entry")

	return rootCmd.Execute()
}

func pack(targets []string, options packageOptions) error {
	if err := os.MkdirAll("package", 0755); err != nil {
		return err
	}

	// tar -czf package/{name}-{version}-{goos}-{goarch}.tar.gz target
	if err := packTarget(targets, options); err != nil {
		return err
	}

	if err := checksum(options); err != nil {
		return err
	}

	if err := manifestIndex(options); err != nil {
		return err
	}

	return componentIndex(options)
}

func current(fname string, target interface{}) error {
	err := load("package/"+fname, target)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) {
		if strings.HasPrefix(mirror, "http") {
			url := join(mirror, fname+"?t="+strconv.Itoa(int(time.Now().Unix())))
			if err := get(url, target); err != nil {
				return err
			}
		} else {
			if utils.IsNotExist(filepath.Join(mirror, fname)) {
				return errNotFound
			}
			f, err := os.Open(filepath.Join(mirror, fname))
			if err != nil {
				return err
			}
			defer f.Close()
			return json.NewDecoder(f).Decode(target)
		}
	}
	return nil
}

func manifestIndex(options packageOptions) error {
	mIndex := v0manifest.ComponentManifest{}
	err := current("tiup-manifest.index", &mIndex)
	if err != nil && err != errNotFound {
		return err
	}
	if err == errNotFound {
		mIndex.Modified = time.Now().Format(time.RFC3339)
		mIndex.TiUPVersion = pkgver.Version(version.NewTiUPVersion().SemVer())
		mIndex.Description = "TiDB components manager"
	}

	pair := options.goos + "/" + options.goarch

	for idx := range mIndex.Components {
		if mIndex.Components[idx].Name == options.name {
			if options.desc != "" {
				mIndex.Components[idx].Desc = options.desc
			}
			if mIndex.Components[idx].Standalone != options.standalone {
				mIndex.Components[idx].Standalone = options.standalone
			}
			if mIndex.Components[idx].Hide != options.hide {
				mIndex.Components[idx].Hide = options.hide
			}
			for _, p := range mIndex.Components[idx].Platforms {
				if p == pair {
					return write("package/tiup-manifest.index", mIndex)
				}
			}
			mIndex.Components[idx].Platforms = append(mIndex.Components[idx].Platforms, pair)
			return write("package/tiup-manifest.index", mIndex)
		}
	}
	mIndex.Components = append(mIndex.Components, v0manifest.ComponentInfo{
		Name:       options.name,
		Desc:       options.desc,
		Standalone: options.standalone,
		Hide:       options.hide,
		Platforms:  []string{pair},
	})

	return write("package/tiup-manifest.index", mIndex)
}

func componentIndex(options packageOptions) error {
	fname := fmt.Sprintf("tiup-component-%s.index", options.name)
	pair := options.goos + "/" + options.goarch

	cIndex := v0manifest.VersionManifest{}
	err := current(fname, &cIndex)
	if err != nil && err != errNotFound {
		return err
	}

	version := pkgver.Version(options.version)

	v := v0manifest.VersionInfo{
		Version:   version,
		Date:      time.Now().Format(time.RFC3339),
		Entry:     options.entry,
		Platforms: []string{pair},
	}

	// Generate a new component index
	if err == errNotFound {
		cIndex = v0manifest.VersionManifest{
			Description: options.desc,
			Modified:    time.Now().Format(time.RFC3339),
		}
		if version.IsNightly() {
			cIndex.Nightly = &v
		} else {
			cIndex.Versions = append(cIndex.Versions, v)
		}
		return write("package/"+fname, cIndex)
	}

	for idx := range cIndex.Versions {
		if cIndex.Versions[idx].Version == version {
			cIndex.Versions[idx].Date = time.Now().Format(time.RFC3339)
			cIndex.Versions[idx].Entry = options.entry
			for _, p := range cIndex.Versions[idx].Platforms {
				if p == pair {
					return write("package/"+fname, cIndex)
				}
			}
			cIndex.Versions[idx].Platforms = append(cIndex.Versions[idx].Platforms, pair)
			return write("package/"+fname, cIndex)
		}
	}

	if version.IsNightly() {
		if cIndex.Nightly == nil {
			cIndex.Nightly = &v
			return write("package/"+fname, cIndex)
		}
		cIndex.Nightly.Date = time.Now().Format(time.RFC3339)
		cIndex.Nightly.Entry = options.entry
		for _, p := range cIndex.Nightly.Platforms {
			if p == pair {
				return write("package/"+fname, cIndex)
			}
		}
		cIndex.Nightly.Platforms = append(cIndex.Nightly.Platforms, pair)
		return write("package/"+fname, cIndex)
	}

	cIndex.Versions = append(cIndex.Versions, v)
	return write("package/"+fname, cIndex)
}

func join(url, file string) string {
	if strings.HasSuffix(url, "/") {
		return url + file
	}
	return url + "/" + file
}

func write(file string, data interface{}) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	return enc.Encode(data)
}

func load(file string, target interface{}) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewDecoder(f).Decode(target)
}

func get(url string, target interface{}) error {
	r, err := http.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	if r.StatusCode == http.StatusNotFound {
		return errNotFound
	}

	return json.NewDecoder(r.Body).Decode(target)
}

func packTarget(targets []string, options packageOptions) error {
	file := fmt.Sprintf("package/%s-%s-%s-%s.tar.gz", options.name, options.version, options.goos, options.goarch)
	args := []string{"-czf", file}
	if options.dir != "" {
		args = append(args, "-C", options.dir)
	}
	cmd := exec.Command("tar", append(args, targets...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Println(cmd.Args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("package target: %s", err.Error())
	}
	return nil
}

func checksum(options packageOptions) error {
	tarball, err := os.OpenFile(fmt.Sprintf("package/%s-%s-%s-%s.tar.gz", options.name, options.version, options.goos, options.goarch), os.O_RDONLY, 0)
	if err != nil {
		return errors.Trace(err)
	}
	defer tarball.Close()

	sha1Writter := sha1.New()
	if _, err := io.Copy(sha1Writter, tarball); err != nil {
		return errors.Trace(err)
	}

	checksum := hex.EncodeToString(sha1Writter.Sum(nil))
	file := fmt.Sprintf("package/%s-%s-%s-%s.sha1", options.name, options.version, options.goos, options.goarch)

	return ioutil.WriteFile(file, []byte(checksum), 0664)
}

func chwd() error {
	pwd, found := os.LookupEnv(localdata.EnvNameWorkDir)
	if !found {
		return fmt.Errorf("cannot get tiup work directory")
	}
	return os.Chdir(pwd)
}
