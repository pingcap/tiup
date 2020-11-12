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

package spec

import (
	"fmt"
	"strings"

	"github.com/tj/go-termd"
)

// ErrorSpec represents the standard error specification introduced by
// https://github.com/pingcap/tidb/blob/master/docs/design/2020-05-08-standardize-error-codes-and-messages.md
type ErrorSpec struct {
	Code        string   `toml:"code" json:"code"`
	Error       string   `toml:"error" json:"error"`
	Description string   `toml:"description" json:"description"`
	Tags        []string `toml:"tags" json:"tags"`
	Workaround  string   `toml:"workaround" json:"workaround"`
	// Used for indexes
	ExtraCode  string `toml:"extracode" json:"extracode"`
	ExtraError string `toml:"extraerror" json:"extraerror"`
}

func newCompiler() *termd.Compiler {
	return &termd.Compiler{
		Columns: 80,
	}
}

// String implements the fmt.Stringer interface
func (f ErrorSpec) String() string {
	var header string
	if len(f.Tags) > 0 {
		header = fmt.Sprintf("# Error: **%s**, Tags: %v", f.Code, f.Tags)
	} else {
		header = fmt.Sprintf("# Error: **%s**", f.Code)
	}

	tmpl := header + "\n" + strings.TrimSpace(f.Error)

	description := f.Description
	if description != "" {
		tmpl += `## Description
		` + strings.TrimSpace(description)
	}
	workaround := f.Workaround
	if workaround != "" {
		tmpl += `## Workaround
		` + strings.TrimSpace(workaround)
	}

	return newCompiler().Compile(tmpl)
}
