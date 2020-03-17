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
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/spf13/cobra"
)

var mirror = "https://tiup-mirrors.pingcap.com/"
var errNotFound = fmt.Errorf("resource not found")

func init() {
	if m := os.Getenv("TIUP_MIRRORS"); m != "" {
		mirror = m
	}
}

func main() {
	if err := execute(); err != nil {
		fmt.Println("Packaging component failed:", err)
		os.Exit(1)
	}
}

func execute() error {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	name := ""
	version := ""
	entry := ""
	desc := ""

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

			return pack(args[0], name, version, entry, goos, goarch, desc)
		},
	}

	rootCmd.Flags().StringVarP(&goos, "os", "", goos, "Target OS of the package")
	rootCmd.Flags().StringVarP(&goarch, "arch", "", goarch, "Target ARCH of the package")
	rootCmd.Flags().StringVarP(&name, "name", "", name, "Name of the package")
	rootCmd.Flags().StringVarP(&version, "release", "", version, "Version of the package")
	rootCmd.Flags().StringVarP(&entry, "entry", "", entry, "Entry point of the package")
	rootCmd.Flags().StringVarP(&desc, "desc", "", desc, "Description of the package")

	rootCmd.MarkFlagRequired("name")
	rootCmd.MarkFlagRequired("release")
	rootCmd.MarkFlagRequired("entry")

	return rootCmd.Execute()
}

func pack(target, name, version, entry, goos, goarch, desc string) error {
	if err := os.MkdirAll("package", 0755); err != nil {
		return err
	}

	// tar -czf package/{name}-{version}-{goos}-{goarch}.tar.gz target
	if err := packTarget(target, name, version, goos, goarch); err != nil {
		return err
	}

	if err := checksum(name, version, goos, goarch); err != nil {
		return err
	}

	if err := manifestIndex(name, desc, goos, goarch); err != nil {
		return err
	}

	if err := componentIndex(name, desc, entry, version, goos, goarch); err != nil {
		return err
	}

	return nil
}

func current(fname string, target interface{}) error {
	err := load("package/"+fname, target)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) {
		url := join(mirror, fname+"?t="+strconv.Itoa(int(time.Now().Unix())))
		if err := get(url, target); err != nil {
			return err
		}
	}
	return nil
}

func manifestIndex(name, desc, goos, goarch string) error {
	mIndex := ManifestIndex{}
	if err := current("tiup-manifest.index", &mIndex); err != nil {
		return err
	}
	pair := goos + "/" + goarch

	for idx := range mIndex.Components {
		if mIndex.Components[idx].Name == name {
			if desc != "" {
				mIndex.Components[idx].Desc = desc
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
	mIndex.Components = append(mIndex.Components, Component{
		Name:      name,
		Desc:      desc,
		Platforms: []string{pair},
	})

	return write("package/tiup-manifest.index", mIndex)
}

func componentIndex(name, desc, entry, version, goos, goarch string) error {
	fname := fmt.Sprintf("tiup-component-%s.index", name)
	pair := goos + "/" + goarch

	cIndex := ComponentIndex{}
	err := current(fname, &cIndex)
	if err != nil && err != errNotFound {
		return err
	}

	v := Version{
		Version:   version,
		Date:      time.Now(),
		Entry:     entry,
		Platforms: []string{pair},
	}

	// Generate a new component index
	if err == errNotFound {
		cIndex = ComponentIndex{
			Description: desc,
			Modified:    time.Now(),
		}
		if version == "nightly" {
			cIndex.Nightly = &v
		} else {
			cIndex.Versions = append(cIndex.Versions, v)
		}
		return write("package/"+fname, cIndex)
	}

	for idx := range cIndex.Versions {
		if cIndex.Versions[idx].Version == version {
			cIndex.Versions[idx].Date = time.Now()
			cIndex.Versions[idx].Entry = entry
			for _, p := range cIndex.Versions[idx].Platforms {
				if p == pair {
					return write("package/"+fname, cIndex)
				}
			}
			cIndex.Versions[idx].Platforms = append(cIndex.Versions[idx].Platforms, pair)
			return write("package/"+fname, cIndex)
		}
	}

	if version == "nightly" {
		if cIndex.Nightly == nil {
			cIndex.Nightly = &v
			return write("package/"+fname, cIndex)
		}
		cIndex.Nightly.Date = time.Now()
		cIndex.Nightly.Entry = entry
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

func packTarget(target, name, version, goos, goarch string) error {
	file := fmt.Sprintf("package/%s-%s-%s-%s.tar.gz", name, version, goos, goarch)
	cmd := exec.Command("tar", "-czf", file, target)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("package target: %s", err.Error())
	}
	return nil
}

func checksum(name, version, goos, goarch string) error {
	tarball, err := os.OpenFile(fmt.Sprintf("package/%s-%s-%s-%s.tar.gz", name, version, goos, goarch), os.O_RDONLY, 0)
	if err != nil {
		return errors.Trace(err)
	}
	defer tarball.Close()

	sha1Writter := sha1.New()
	if _, err := io.Copy(sha1Writter, tarball); err != nil {
		return errors.Trace(err)
	}

	checksum := hex.EncodeToString(sha1Writter.Sum(nil))
	file := fmt.Sprintf("package/%s-%s-%s-%s.sha1", name, version, goos, goarch)
	if err := ioutil.WriteFile(file, []byte(checksum), 0664); err != nil {
		return err
	}
	return nil
}

func chwd() error {
	pwd := os.Getenv("PWD")
	if pwd == "" {
		return fmt.Errorf("Env PWD not set")
	}

	return os.Chdir(pwd)
}
