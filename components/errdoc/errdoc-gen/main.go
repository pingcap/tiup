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
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/pingcap/tiup/components/errdoc/spec"
)

var opt struct {
	source string
}

func init() {
	flag.StringVar(&opt.source, "source", "", "The source directory of error documentation")
}

func log(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}

func fatal(format string, args ...interface{}) {
	log(format, args...)
	os.Exit(1)
}

func main() {
	flag.Parse()
	if opt.source == "" {
		fatal("The source directory cannot be empty")
	}

	errorSpecs, err := errdoc(opt.source)
	if err != nil {
		log("Extract the error documentation failed: %v", err)
	}

	fmt.Println(errorSpecs)
}

func errdoc(source string) ([]*spec.ErrorSpec, error) {
	source, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}

	var errorSpecs []*spec.ErrorSpec
	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		// Only export the global errors defined by `errors.Normalize` (github.com/pingcap/errors)
		var alias string
		for _, imp := range file.Imports {
			if strings.Trim(imp.Path.Value, "\"") == "github.com/pingcap/errors" {
				if imp.Name != nil {
					alias = imp.Name.Name
				} else {
					alias = "errors"
				}
				break
			}
		}
		// Doesn't import the `github.com/pingcap/errors`
		if alias == "" {
			return nil
		}
		// Export the standard errors
		{
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return err
			}
			errorSpecs = append(errorSpecs, export(alias, file)...)
		}
		return nil
	})
	return errorSpecs, err
}

func export(alias string, f *ast.File) []*spec.ErrorSpec {
	if len(f.Decls) == 0 {
		return nil
	}

	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || len(gen.Specs) == 0 {
			continue
		}
		for _, spec := range gen.Specs {
			switch errSpec := spec.(type) {
			case *ast.ValueSpec:
				// CASES:
				// var ErrXXX = errors.Normalize(string, opts...)
				// var (
				//     ErrYYY = errors.Normalize(string, opts...)
				//     ErrZZZ = errors.Normalize(string, opts...)
				//     A = errors.Normalize(string, opts...)
				// )
				// var ErrXXX, ErrYYY = errors.Normalize(string, opts...), errors.Normalize(string, opts...)
				// var (
				//     ErrYYY = errors.Normalize(string, opts...)
				//     ErrZZZ, ErrWWW = errors.Normalize(string, opts...), errors.Normalize(string, opts...)
				//     A = errors.Normalize(string, opts...)
				// )
				//
				for i, name := range errSpec.Names {
					if !strings.HasPrefix(name.Name, "Err") {
						continue
					}
					val, ok := errSpec.Values[i].(*ast.CallExpr)
					if !ok {
						continue
					}
					log("---> %#v ==> %#v", name, val)
				}
			default:
				continue
			}
		}
	}
	return nil
}
