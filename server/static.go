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
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"

	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
)

// mirrorFileSystem excludes signing-key directories, symlinks and directory listings.
type mirrorFileSystem struct {
	root        string
	privateDirs []string
}

func (fs mirrorFileSystem) Open(name string) (http.File, error) {
	local, err := filepath.Abs(fs.root)
	if err != nil {
		return nil, err
	}
	relative := filepath.FromSlash(strings.TrimPrefix(path.Clean("/"+name), "/"))
	if relative == "" {
		return nil, os.ErrPermission
	}
	file := filepath.Join(local, relative)
	privateDirs := append([]string{filepath.Join(local, "keys")}, fs.privateDirs...)
	for i, privateDir := range privateDirs {
		private, err := filepath.Abs(privateDir)
		if err != nil {
			return nil, err
		}
		privateDirs[i] = private
		if pathWithin(private, file) {
			return nil, os.ErrPermission
		}
	}
	root, err := filepath.EvalSymlinks(local)
	if err != nil {
		// Unavailable mirror storage is not a missing public file: do not let
		// os.IsNotExist classify this error as eligible for upstream fallback.
		return nil, fmt.Errorf("resolve mirror root: %v", err)
	}
	file = filepath.Join(root, relative)
	var privateInfos []os.FileInfo
	for _, private := range privateDirs {
		if pathWithin(private, file) {
			return nil, os.ErrPermission
		}
		resolved, err := resolvePrivateDirectory(private)
		if err != nil {
			return nil, err
		}
		if pathWithin(resolved, file) {
			return nil, os.ErrPermission
		}
		info, err := os.Stat(private)
		if err == nil {
			privateInfos = append(privateInfos, info)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open mirror root: %v", err)
	}
	defer func() { _ = directory.Close() }()
	// Mirror entries are managed by trusted writers. Reject static aliases here;
	// this policy does not synchronize with concurrent path replacement.
	current := ""
	parts := strings.Split(relative, string(filepath.Separator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := directory.Lstat(current)
		if err != nil {
			return nil, err
		}
		last := i == len(parts)-1
		if (last && !info.Mode().IsRegular()) || (!last && !info.IsDir()) {
			return nil, os.ErrPermission
		}
		// Keep the identity guard for filesystem aliases beyond the explicit
		// Unicode and case equivalences recognized by the lexical policy.
		for _, privateInfo := range privateInfos {
			if os.SameFile(privateInfo, info) {
				return nil, os.ErrPermission
			}
		}
	}
	return directory.Open(relative)
}

// resolvePrivateDirectory resolves existing parents even when the key directory
// has not been created. Otherwise an external symlink to the mirror can hide a
// configured missing private path from the upstream-fallback policy.
func resolvePrivateDirectory(dir string) (string, error) {
	suffix := ""
	for {
		resolved, err := filepath.EvalSymlinks(dir)
		if err == nil {
			return filepath.Join(resolved, suffix), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		_, statErr := os.Lstat(dir)
		if statErr == nil || !os.IsNotExist(statErr) || filepath.Dir(dir) == dir {
			// An existing but unresolvable prefix (for example a dangling
			// symlink) is a configuration failure, not a public file miss.
			return "", fmt.Errorf("resolve private directory: %v", err)
		}
		suffix = filepath.Join(filepath.Base(dir), suffix)
		dir = filepath.Dir(dir)
	}
}

func pathWithin(dir, file string) bool {
	// Compare canonical Unicode and case variants even when the local directory
	// is absent: the upstream may resolve them to keys. These folded paths are
	// only for the access policy, never for locating files on disk.
	fold := cases.Fold()
	// Canonical caseless matching (Unicode D145) normalizes before folding too:
	// folding U+0345 turns a combining mark into a starter, affecting reordering.
	canonical := func(name string) string {
		return norm.NFD.String(fold.String(norm.NFD.String(name)))
	}
	relative, err := filepath.Rel(canonical(dir), canonical(file))
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// staticServer starts a static web server.
func staticServer(local string, upstream string, privateDirs ...string) http.Handler {
	fs := mirrorFileSystem{root: local, privateDirs: privateDirs}
	fsh := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f, err := fs.Open(path.Clean(r.URL.Path)); err == nil {
			f.Close()
		} else if os.IsNotExist(err) && upstream != "" {
			if err := proxyUpstream(w, r, path.Join(local, path.Clean(r.URL.Path)), upstream); err != nil {
				logprinter.Errorf("Proxy upstream: %s", err.Error())
				fsh.ServeHTTP(w, r)
			}
			logprinter.Errorf("Handle file: %s", err.Error())
			return
		}
		fsh.ServeHTTP(w, r)
	})
}

func proxyUpstream(w http.ResponseWriter, r *http.Request, file, upstream string) error {
	url, err := url.Parse(upstream)
	if err != nil {
		return err
	}

	r.Host = url.Host
	r.URL.Host = url.Host
	r.URL.Scheme = url.Scheme

	httputil.NewSingleHostReverseProxy(url).ServeHTTP(w, r)
	return nil
}

func (s *server) static(prefix, root, upstream string) http.Handler {
	return http.StripPrefix(prefix, staticServer(root, upstream, s.keyDir))
}
