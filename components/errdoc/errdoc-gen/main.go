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
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pingcap/tiup/pkg/utils"
)

var opt struct {
	source string
	module string
	output string
}

func init() {
	flag.StringVar(&opt.source, "source", "", "The source directory of error documentation")
	flag.StringVar(&opt.module, "module", "", "The module name of target repository")
	flag.StringVar(&opt.output, "output", "", "The output path of error documentation file")
}

func log(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}

func fatal(format string, args ...interface{}) {
	log(format, args...)
	os.Exit(1)
}

const autoDirectoryName = "_errdoc-generator"
const entryFileName = "main.go"

func main() {
	flag.Parse()
	if opt.source == "" {
		fatal("The source directory cannot be empty")
	}

	source, err := filepath.EvalSymlinks(opt.source)
	if err != nil {
		fatal("Evaluate symbol link path %s failed: %v", opt.source, err)
	}
	opt.source = source

	if !utils.IsExist(filepath.Join(opt.source, "go.mod")) {
		fatal("The source directory is not the root path of codebase(go.mod not found)")
	}

	if opt.output == "" {
		opt.output = filepath.Join(opt.source, "errors.toml")
		log("The --output argument is missing, default to %s", opt.output)
	}

	errNames, err := errdoc(opt.source, opt.module)
	if err != nil {
		log("Extract the error documentation failed: %+v", err)
	}

	targetDir := filepath.Join(opt.source, autoDirectoryName)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		fatal("Cannot create the errdoc: %+v", err)
	}
	defer os.RemoveAll(targetDir)

	tmpl := `
package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"reflect"
	"fmt"
	"sort"

	"github.com/pingcap/errors"
{{- range $decl := .}}
	{{$decl.PackageName}} "{{- $decl.ImportPath}}"
{{- end}}
)

func main() {
	var outpath string
	flag.StringVar(&outpath, "output", "", "Specify the error documentation output file path")
	flag.Parse()
	if outpath == "" {
		println("Usage: ./_errdoc-generator --out /path/to/errors.toml")
		os.Exit(1)
	}
	
	var allErrors []error
	{{- range $decl := .}}
		{{- range $err := $decl.ErrNames}}
			allErrors = append(allErrors, {{$decl.PackageName}}.{{- $err}})
		{{- end}}
	{{- end}}
	
	type spec struct {
		Code        string
		Message     string
		Description string
		Workaround  string
	}
	var sorted []spec
	for _, e := range allErrors {
		terr, ok := e.(*errors.Error)
		if !ok {
			println("Non-normalized error:", e)
		} else {
			val := reflect.ValueOf(terr).Elem()
			codeText := val.FieldByName("codeText")
			message := val.FieldByName("message")
			description := val.FieldByName("description")
			workaround := val.FieldByName("workaround ")
			s := spec{
				Code:    codeText.String(),
				Message: message.String(),
			}
			if description.IsValid() {
				s.Description = description.String()
			}
			if workaround.IsValid() {
				s.Workaround = workaround.String()
			}
			sorted = append(sorted, s)
		}
	}

	sort.Slice(sorted, func(i, j int) bool {
		// TiDB exits duplicated code
		if sorted[i].Code == sorted[j].Code {
			return sorted[i].Message < sorted[j].Message
		}
		return sorted[i].Code < sorted[j].Code
	})

	// We don't use toml library to serialize it due to cannot reserve the order for map[string]spec
	buffer := bytes.NewBufferString("# AUTOGENERATED BY github.com/pingcap/tiup/components/errdoc/errdoc-gen\n" +
		"# DO NOT EDIT THIS FILE, PLEASE CHANGE ERROR DEFINITION IF CONTENT IMPROPER.\n\n")
	for _, item := range sorted {
		buffer.WriteString(fmt.Sprintf("[\"%s\"]\nerror = \"%s\"\n", item.Code, item.Message))
		if item.Description != "" {
			buffer.WriteString(fmt.Sprintf("description = '''%s'''\n", item.Description))
		}
		if item.Workaround != "" {
			buffer.WriteString(fmt.Sprintf("workaround = '''%s'''\n", item.Workaround))
		}
		buffer.WriteString("\n")
	}
	if err := ioutil.WriteFile(outpath, buffer.Bytes(), os.ModePerm); err != nil {
		panic(err)
	}
}
`

	t, err := template.New("_errdoc-template").Parse(tmpl)
	if err != nil {
		fatal("Parse template failed: %+v", err)
	}

	outFile := filepath.Join(targetDir, entryFileName)
	out, err := os.OpenFile(outFile, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		fatal("Open %s failed: %+v", outFile, err)
	}
	defer out.Close()

	if err := t.Execute(out, errNames); err != nil {
		fatal("Render template failed: %+v", err)
	}

	output, err := filepath.Abs(opt.output)
	if err != nil {
		fatal("Evaluate the absolute path of output failed: %+v", err)
	}

	cmd := exec.Command("go", "run", filepath.Join(autoDirectoryName, entryFileName), "--output", output)
	cmd.Dir = opt.source
	data, err := cmd.CombinedOutput()
	if err != nil {
		fatal("Generate %s failed: %v, output:\n%s", opt.output, err, string(data))
	}
}

type errDecl struct {
	ImportPath  string
	PackageName string
	ErrNames    []string
}

func errdoc(source, module string) ([]*errDecl, error) {
	source, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}

	dedup := map[string]*errDecl{}

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
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		errNames := export(file)
		if len(errNames) < 1 {
			return nil
		}
		dirPath := filepath.Dir(path)
		subPath, err := filepath.Rel(source, dirPath)
		if err != nil {
			return err
		}
		packageName := strings.ReplaceAll(subPath, "/", "_")
		if decl, found := dedup[packageName]; found {
			decl.ErrNames = append(decl.ErrNames, errNames...)
		} else {
			decl := &errDecl{
				ImportPath:  filepath.Join(module, subPath),
				PackageName: packageName,
				ErrNames:    errNames,
			}
			dedup[packageName] = decl
		}
		return nil
	})

	var errDecls []*errDecl
	for _, decl := range dedup {
		errDecls = append(errDecls, decl)
	}

	return errDecls, err
}

func export(f *ast.File) []string {
	if len(f.Decls) == 0 {
		return nil
	}

	var errNames []string
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
				if len(errSpec.Names) != len(errSpec.Values) {
					continue
				}
				for i, name := range errSpec.Names {
					if !strings.HasPrefix(name.Name, "Err") {
						continue
					}
					_, ok := errSpec.Values[i].(*ast.CallExpr)
					if !ok {
						continue
					}
					errNames = append(errNames, name.Name)
				}
			default:
				continue
			}
		}
	}
	return errNames
}
