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
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis/lang/en"
	_ "github.com/blevesearch/bleve/index/store/goleveldb"
	"github.com/blevesearch/bleve/mapping"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/spf13/cobra"
	"github.com/tj/go-termd"
)

func init() {
	bleve.Config.DefaultKVStore = "goleveldb"
}

func main() {
	rootCmd := &cobra.Command{
		Use:          "tiup err",
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

type errorSpec struct {
	Code        string   `toml:"code" json:"code"`
	Message     string   `toml:"message" json:"message"`
	Description string   `toml:"description" json:"description"`
	Tags        []string `toml:"tags" json:"tags"`
	Workaround  string   `toml:"workaround" json:"workaround"`
}

func newCompiler() *termd.Compiler {
	return &termd.Compiler{
		Columns: 80,
	}
}

// String implements the fmt.Stringer interface
func (f errorSpec) String() string {
	var header string
	if len(f.Tags) > 0 {
		header = fmt.Sprintf("# Error: **%s**, Tags: %v", f.Code, f.Tags)
	} else {
		header = fmt.Sprintf("# Error: **%s**", f.Code)
	}

	return newCompiler().Compile(fmt.Sprintf(`%s
%s
## Description
%s
## Workaround
%s`, header, f.Message, f.Description, f.Workaround))
}

func searchError(args []string) error {
	index, errStore, err := loadIndex()
	if err != nil {
		return err
	}

	result, err := index.Search(bleve.NewSearchRequest(bleve.NewMatchPhraseQuery(strings.Join(args, " "))))
	if err != nil {
		return err
	}

	for i, match := range result.Hits {
		spec := errStore[match.ID]
		fmt.Println(spec)
		if i != len(result.Hits)-1 {
			fmt.Println(string(bytes.Repeat([]byte("-"), 80)))
		}
	}

	return nil
}

func loadIndex() (bleve.Index, map[string]*errorSpec, error) {
	dir := os.Getenv(localdata.EnvNameComponentInstallDir)
	if dir == "" {
		return nil, nil, errors.New("component `doc` doesn't running in TiUP mode")
	}

	type tomlFile struct {
		Errors map[string]*errorSpec `toml:"error"`
	}
	indexPath := filepath.Join(dir, "index")

	index, err := bleve.Open(indexPath)
	if err != nil && err != bleve.ErrorIndexPathDoesNotExist {
		return nil, nil, err
	}

	needIndex := err == bleve.ErrorIndexPathDoesNotExist

	if needIndex {
		indexMapping := buildIndexMapping()
		if err := os.MkdirAll(indexPath, 0644); err != nil {
			return nil, nil, err
		}
		index, err = bleve.New(indexPath, indexMapping)
		if err != nil {
			return nil, nil, err
		}
	}

	errStore := map[string]*errorSpec{}

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
		if _, err := toml.DecodeReader(reader, &file); err != nil {
			return nil
		}
		for code, spec := range file.Errors {
			spec.Code = code
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

func buildIndexMapping() mapping.IndexMapping {
	englishTextFieldMapping := bleve.NewTextFieldMapping()
	englishTextFieldMapping.Analyzer = en.AnalyzerName

	documentMapping := bleve.NewDocumentMapping()

	documentMapping.AddFieldMappingsAt("message", englishTextFieldMapping)
	documentMapping.AddFieldMappingsAt("description", englishTextFieldMapping)
	documentMapping.AddFieldMappingsAt("workaround", englishTextFieldMapping)
	documentMapping.AddFieldMappingsAt("tags", englishTextFieldMapping)
	documentMapping.AddFieldMappingsAt("code", englishTextFieldMapping)

	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = documentMapping
	indexMapping.TypeField = "type"
	indexMapping.DefaultAnalyzer = "en"

	return indexMapping
}
