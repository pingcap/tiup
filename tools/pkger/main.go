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
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type options struct {
	Source      string
	Destination string
	Package     string
}

func main() {
	opt := options{}
	rootCmd := &cobra.Command{
		Use:          "pkger",
		Short:        "Package the files into source code",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generate(opt)
		},
	}

	rootCmd.Flags().StringVarP(&opt.Source, "source", "s", "", "The source directory path")
	rootCmd.Flags().StringVarP(&opt.Destination, "destination", "d", "", "The destination directory path")
	rootCmd.Flags().StringVarP(&opt.Package, "package", "p", "", "The package name of destination")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func generate(opt options) error {
	if opt.Source == "" {
		return errors.New("source directory must be specified")
	}
	if opt.Destination == "" {
		return errors.New("destination directory must be specified")
	}
	if opt.Package == "" {
		opt.Package = filepath.Base(opt.Destination)
	}

	source, err := filepath.Abs(opt.Source)
	if err != nil {
		return err
	}
	dest, err := filepath.Abs(opt.Destination)
	if err != nil {
		return err
	}

	parentDir := filepath.Dir(source)

	fmt.Println("Source directory", source)
	fmt.Println("Source parent directory", parentDir)
	fmt.Println("destination directory", dest)
	fmt.Println("package name", opt.Package)

	files := map[string]string{}
	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return err
		}

		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		subpath := path[len(parentDir):]
		fmt.Println("Found file", subpath)

		files[subpath] = base64.StdEncoding.EncodeToString(content)
		return nil
	})
	if err != nil {
		return err
	}

	// Sort by filename
	var filenames []string
	for n := range files {
		filenames = append(filenames, n)
	}
	sort.Strings(filenames)

	var lines []string
	for _, n := range filenames {
		lines = append(lines, fmt.Sprintf(`autogenFiles["%s"] = "%s"`, n, files[n]))
	}

	writeBack := fmt.Sprintf(`// Copyright 2020 PingCAP, Inc.
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

package %s

var autogenFiles = map[string]string{}

func init() {
	%s
}
`, opt.Package, strings.Join(lines, "\n\t"))

	return ioutil.WriteFile(filepath.Join(dest, "autogen_pkger.go"), []byte(writeBack), 0755)
}
