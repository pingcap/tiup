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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/blevesearch/bleve"
	_ "github.com/blevesearch/bleve/index/store/goleveldb"
	"github.com/blevesearch/bleve/search/query"
	"github.com/pingcap/tiup/components/errdoc/spec"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/spf13/cobra"
)

func init() {
	bleve.Config.DefaultKVStore = "goleveldb"
}

func main() {
	rootCmd := &cobra.Command{
		Use:          "tiup errdoc",
		Short:        "Show detailed error message via error code or keyword",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return searchError(args)
		},
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func searchError(args []string) error {
	index, errStore, err := loadIndex()
	if err != nil {
		return err
	}

	// Bleve search
	queries := []query.Query{
		bleve.NewMatchPhraseQuery(strings.Join(args, " ")),
	}
	var terms []query.Query
	var prefix []query.Query
	for _, arg := range args {
		terms = append(terms, bleve.NewTermQuery(arg))
		prefix = append(prefix, bleve.NewPrefixQuery(arg))
	}
	queries = append(queries, bleve.NewConjunctionQuery(terms...))
	queries = append(queries, bleve.NewConjunctionQuery(prefix...))

	result, err := index.Search(bleve.NewSearchRequest(bleve.NewDisjunctionQuery(queries...)))
	if err != nil {
		return err
	}

	all := map[string]struct{}{}
	for _, match := range result.Hits {
		all[match.ID] = struct{}{}
	}

	// Error code prefix match
	if len(args) == 1 {
		c := strings.ToLower(args[0])
		for code := range errStore {
			if strings.HasPrefix(strings.ToLower(code), c) {
				all[code] = struct{}{}
			}
		}
	}

	var sorted []string
	for code := range all {
		sorted = append(sorted, code)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	for _, code := range sorted {
		spec := errStore[code]
		fmt.Println(spec)
	}

	fmt.Printf("%d matched\n", len(sorted))

	return nil
}

func loadIndex() (bleve.Index, map[string]*spec.ErrorSpec, error) {
	dir := os.Getenv(localdata.EnvNameComponentInstallDir)
	if dir == "" {
		return nil, nil, errors.New("component `errdoc` doesn't running in TiUP mode")
	}

	type tomlFile map[string]*spec.ErrorSpec
	indexPath := filepath.Join(dir, "index")

	index, err := bleve.Open(indexPath)
	if err != nil && err != bleve.ErrorIndexPathDoesNotExist {
		return nil, nil, err
	}

	needIndex := err == bleve.ErrorIndexPathDoesNotExist

	if needIndex {
		indexMapping := bleve.NewIndexMapping()
		if err := os.MkdirAll(indexPath, 0755); err != nil {
			return nil, nil, err
		}
		index, err = bleve.New(indexPath, indexMapping)
		if err != nil {
			return nil, nil, err
		}
	}

	errStore := map[string]*spec.ErrorSpec{}

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), "toml") {
			return nil
		}
		reader, err := os.Open(path)
		if err != nil {
			return err
		}
		// Ignore file if cannot be unmarshalled to error specification
		file := tomlFile{}
		if _, err := toml.NewDecoder(reader).Decode(&file); err != nil {
			return nil
		}
		for code, spec := range file {
			spec.Code = code
			spec.ExtraCode = strings.ReplaceAll(code, ":", " ")
			spec.ExtraError = strings.ReplaceAll(spec.Error, ":", " ")
			errStore[code] = spec
			if !needIndex {
				continue
			}
			if err := index.Index(code, spec); err != nil {
				return err
			}
		}
		return nil
	})
	return index, errStore, err
}
