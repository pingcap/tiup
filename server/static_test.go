// Copyright 2026 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStaticServerPrivateFiles(t *testing.T) {
	for _, keyLocation := range []string{"default", "custom", "external", "symlink-root"} {
		t.Run(keyLocation, func(t *testing.T) {
			root := t.TempDir()
			if keyLocation == "symlink-root" {
				alias := filepath.Join(t.TempDir(), "mirror")
				require.NoError(t, os.Symlink(root, alias))
				root = alias
			}
			keyDir := filepath.Join(root, "keys")
			switch keyLocation {
			case "custom":
				keyDir = filepath.Join(root, "private-signing")
			case "external":
				keyDir = t.TempDir()
			}
			require.NoError(t, os.MkdirAll(keyDir, 0700))
			var upstreamRequests atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamRequests.Add(1)
				_, _ = w.Write([]byte("upstream public file"))
			}))
			defer upstream.Close()
			configuredKeyDir := keyDir
			if keyLocation == "default" || keyLocation == "symlink-root" {
				configuredKeyDir = ""
			}
			s, err := newServer(root, configuredKeyDir, upstream.URL)
			require.NoError(t, err)
			// Use inert markers, not signing keys or executable packages.
			require.NoError(t, os.WriteFile(filepath.Join(keyDir, "private.json"), []byte("PRIVATE TEST MARKER"), 0600))
			require.NoError(t, os.MkdirAll(filepath.Join(root, "keys"), 0700))
			require.NoError(t, os.WriteFile(filepath.Join(root, "keys", "legacy.json"), []byte("LEGACY PRIVATE MARKER"), 0600))
			require.NoError(t, os.WriteFile(filepath.Join(root, "root.json"), []byte("public metadata"), 0600))
			require.NoError(t, os.Mkdir(filepath.Join(root, "public"), 0700))
			require.NoError(t, os.WriteFile(filepath.Join(root, "public", "component.tar.gz"), []byte("public archive"), 0600))
			// Directory denial must also apply when an index page exists.
			require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte("root index"), 0600))
			require.NoError(t, os.WriteFile(filepath.Join(root, "public", "index.html"), []byte("public index"), 0600))
			require.NoError(t, os.Symlink(filepath.Join(keyDir, "private.json"), filepath.Join(root, "key-alias.json")))
			require.NoError(t, os.Symlink(keyDir, filepath.Join(root, "key-alias-dir")))
			require.NoError(t, os.Symlink(filepath.Join("keys", "legacy.json"), filepath.Join(root, "relative-key")))
			external := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(external, "other.json"), []byte("OUTSIDE ROOT MARKER"), 0600))
			require.NoError(t, os.Symlink(filepath.Join(external, "other.json"), filepath.Join(root, "outside-alias.json")))
			h := s.router()
			blocked := []string{"/keys", "/keys/", "/keys/legacy.json", "/keys/missing.json", "/KEYS/legacy.json", "/KEYS/missing.json", "/%6beys/legacy.json", "/key-alias.json", "/key-alias-dir/private.json", "/key-alias-dir/missing.json", "/relative-key", "/outside-alias.json", "/", "/public/"}
			if keyLocation == "custom" {
				blocked = append(blocked, "/private-signing/private.json", "/private-signing/missing.json", "/PRIVATE-SIGNING/missing.json")
			}
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				for _, target := range blocked {
					t.Run(method+target, func(t *testing.T) {
						w := httptest.NewRecorder()
						h.ServeHTTP(w, httptest.NewRequest(method, target, nil))
						require.Equal(t, http.StatusForbidden, w.Code)
						require.NotContains(t, w.Body.String(), "MARKER")
						require.Zero(t, upstreamRequests.Load(), "private paths must not reach the upstream")
					})
				}
			}
			for target, body := range map[string]string{"/root.json": "public metadata", "/public/component.tar.gz": "public archive", "/missing-public.json": "upstream public file"} {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
				require.Equal(t, http.StatusOK, w.Code)
				require.Equal(t, body, w.Body.String())
			}
			require.Equal(t, int32(1), upstreamRequests.Load())
		})
	}
}

func TestStaticServerKeyDirectoryAliases(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "signing"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "signing", "private.json"), []byte("PRIVATE MARKER"), 0600))
	alias := filepath.Join(t.TempDir(), "keys")
	require.NoError(t, os.Symlink(filepath.Join(root, "signing"), alias))
	for _, keyDir := range []string{alias, filepath.Join(root, "public", "..", "signing"), root} {
		h := staticServer(root, "", keyDir)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/signing/private.json", nil))
		require.Equal(t, http.StatusForbidden, w.Code, keyDir)
		require.NotContains(t, w.Body.String(), "MARKER")
	}
}

func TestStaticServerPublicHTTP(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "keys-public"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "keys-public", "package.tar.gz"), []byte("public archive"), 0600))
	require.NoError(t, os.Symlink(filepath.Join(root, "keys-public", "package.tar.gz"), filepath.Join(root, "public-link")))
	h := staticServer(root, "")
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "/keys-public/package.tar.gz", nil))
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "14", w.Header().Get("Content-Length"))
		if method == http.MethodGet {
			require.Equal(t, "public archive", w.Body.String())
		} else {
			require.Empty(t, w.Body.String())
		}
	}
	r := httptest.NewRequest(http.MethodGet, "/keys-public/package.tar.gz", nil)
	r.Header.Set("Range", "bytes=0-5")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusPartialContent, w.Code)
	require.Equal(t, "public", w.Body.String())
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/public-link", nil))
	require.Equal(t, http.StatusForbidden, w.Code, "public symlinks are deliberately unsupported")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/missing.json", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestStaticServerMissingPrivateDirectories(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "mirror")
	require.NoError(t, os.Symlink(root, alias))
	for _, root := range []string{root, alias} {
		for _, keyDir := range []string{"", filepath.Join(root, "private-signing")} {
			s, err := newServer(root, keyDir, "://invalid-upstream")
			require.NoError(t, err)
			for _, target := range []string{"/keys/missing.json", "/KEYS/missing.json", "/private-signing/missing.json"} {
				if keyDir == "" && target == "/private-signing/missing.json" {
					continue
				}
				w := httptest.NewRecorder()
				s.router().ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
				require.Equal(t, http.StatusForbidden, w.Code)
			}
		}
	}
}

func TestStaticServerUnicodeKeyDirectoryAlias(t *testing.T) {
	root := t.TempDir()
	keyDir := filepath.Join(root, "caf\u00e9")
	alias := "cafe\u0301"
	require.NoError(t, os.Mkdir(keyDir, 0700))
	privateInfo, err := os.Stat(keyDir)
	require.NoError(t, err)
	aliasInfo, err := os.Stat(filepath.Join(root, alias))
	if os.IsNotExist(err) {
		t.Skip("filesystem does not alias NFC and NFD directory names")
	}
	require.NoError(t, err)
	require.True(t, os.SameFile(privateInfo, aliasInfo))

	var upstreamRequests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamRequests.Add(1)
		_, _ = w.Write([]byte("upstream public file"))
	}))
	defer upstream.Close()
	s, err := newServer(root, keyDir, upstream.URL)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(keyDir, "private.json"), []byte("PRIVATE TEST MARKER"), 0600))
	h := s.router()
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "/"+url.PathEscape(alias)+"/private.json", nil))
		require.Equal(t, http.StatusForbidden, w.Code)
		require.NotContains(t, w.Body.String(), "MARKER")
		require.Zero(t, upstreamRequests.Load())
	}
}

func TestStaticServerPrivatePathNormalization(t *testing.T) {
	for _, keyName := range []string{"keys", "private-signing"} {
		t.Run(keyName, func(t *testing.T) {
			root := t.TempDir()
			keyDir := filepath.Join(root, keyName)
			require.NoError(t, os.Mkdir(keyDir, 0700))
			require.NoError(t, os.Mkdir(filepath.Join(root, "public"), 0700))
			var upstreamRequests atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamRequests.Add(1)
				_, _ = w.Write([]byte("upstream public file"))
			}))
			defer upstream.Close()
			configuredKeyDir := keyDir
			if keyName == "keys" {
				configuredKeyDir = ""
			}
			s, err := newServer(root, configuredKeyDir, upstream.URL)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(keyDir, "private.json"), []byte("PRIVATE TEST MARKER"), 0600))
			server := httptest.NewServer(s.router())
			defer server.Close()
			client := server.Client()
			client.Timeout = 5 * time.Second
			// Use a real client so mux canonicalization redirects are followed.
			paths := []struct{ name, path string }{
				{"dot-segment", "/public/../" + keyName + "/private.json"},
				{"encoded-dot-segment", "/public/%2e%2e/" + keyName + "/private.json"},
				{"encoded-parent-separator", "/public/..%2f" + keyName + "/private.json"},
				{"encoded-key-separator", "/" + keyName + "%2fprivate.json"},
				{"repeated-slashes", "//" + keyName + "//private.json"},
			}
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				for _, target := range paths {
					t.Run(method+"/"+target.name, func(t *testing.T) {
						r, err := http.NewRequest(method, server.URL+target.path, nil)
						require.NoError(t, err)
						response, err := client.Do(r)
						require.NoError(t, err)
						body, err := io.ReadAll(response.Body)
						require.NoError(t, response.Body.Close())
						require.NoError(t, err)
						require.Equal(t, http.StatusForbidden, response.StatusCode)
						require.NotContains(t, string(body), "MARKER")
						require.Zero(t, upstreamRequests.Load())
					})
				}
			}
		})
	}
}

func TestStaticServerUnavailableRoot(t *testing.T) {
	for _, symlinkRoot := range []bool{false, true} {
		for _, keyName := range []string{"keys", "private-signing"} {
			realRoot := filepath.Join(t.TempDir(), "mirror")
			require.NoError(t, os.Mkdir(realRoot, 0700))
			root := realRoot
			if symlinkRoot {
				root = filepath.Join(t.TempDir(), "alias")
				require.NoError(t, os.Symlink(realRoot, root))
			}
			var upstreamRequests atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamRequests.Add(1)
				_, _ = w.Write([]byte("UPSTREAM MARKER"))
			}))
			t.Cleanup(upstream.Close)
			s, err := newServer(root, filepath.Join(root, keyName), upstream.URL)
			require.NoError(t, err)
			h := s.router()
			// The root becomes unavailable before requests, with no concurrent writer.
			require.NoError(t, os.Rename(realRoot, realRoot+"-offline"))
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				for target, status := range map[string]int{
					"/keys/private.json":            http.StatusForbidden,
					"/" + keyName + "/private.json": http.StatusForbidden,
					"/root.json":                    http.StatusInternalServerError,
				} {
					w := httptest.NewRecorder()
					h.ServeHTTP(w, httptest.NewRequest(method, target, nil))
					require.Equal(t, status, w.Code, "%s %s, symlink=%v", method, target, symlinkRoot)
					require.NotContains(t, w.Body.String(), "MARKER")
					require.Zero(t, upstreamRequests.Load())
				}
			}
			require.NoError(t, os.Rename(realRoot+"-offline", realRoot))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/missing-public.json", nil))
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, "UPSTREAM MARKER", w.Body.String())
			require.Equal(t, int32(1), upstreamRequests.Load())
			upstream.Close()
		}
	}
}

func TestStaticServerMissingPrivateAliases(t *testing.T) {
	for _, location := range []string{"unicode", "case-fold", "combining-mark-order", "symlink-parent", "dangling-parent"} {
		t.Run(location, func(t *testing.T) {
			root := t.TempDir()
			keyDir := filepath.Join(root, "caf\u00e9")
			aliases := []string{"caf\u00e9", "cafe\u0301", "CAFE\u0301"}
			switch location {
			case "case-fold":
				keyDir = filepath.Join(root, "\u03c3")
				aliases = []string{"\u03c3", "\u03c2", "\u03a3"}
			case "combining-mark-order":
				keyDir = filepath.Join(root, "\u1fb4")
				aliases = []string{"\u1fb4", "\u03b1\u0345\u0301"}
			case "symlink-parent", "dangling-parent":
				parent := filepath.Join(t.TempDir(), "alias")
				target := root
				if location == "dangling-parent" {
					target = filepath.Join(root, "not-created")
				}
				require.NoError(t, os.Symlink(target, parent))
				keyDir = filepath.Join(parent, "nested", "private-signing")
				aliases = []string{"nested/private-signing"}
			}
			var upstreamRequests atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamRequests.Add(1)
				_, _ = w.Write([]byte("UPSTREAM MARKER"))
			}))
			defer upstream.Close()
			s, err := newServer(root, keyDir, upstream.URL)
			require.NoError(t, err)
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				for _, alias := range aliases {
					target := "/" + url.PathEscape(alias) + "/private.json"
					w := httptest.NewRecorder()
					s.router().ServeHTTP(w, httptest.NewRequest(method, target, nil))
					status := http.StatusForbidden
					if location == "dangling-parent" {
						status = http.StatusInternalServerError
					}
					require.Equal(t, status, w.Code, "%s %s", method, target)
					require.NotContains(t, w.Body.String(), "MARKER")
					require.Zero(t, upstreamRequests.Load())
				}
			}
			w := httptest.NewRecorder()
			s.router().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/missing-public.json", nil))
			if location == "dangling-parent" {
				require.Equal(t, http.StatusInternalServerError, w.Code)
				require.Zero(t, upstreamRequests.Load())
			} else {
				require.Equal(t, http.StatusOK, w.Code)
				require.Equal(t, "UPSTREAM MARKER", w.Body.String())
				require.Equal(t, int32(1), upstreamRequests.Load())
			}
		})
	}
}

func TestStaticServerSymlinkRootCaseAlias(t *testing.T) {
	realRoot := filepath.Join(t.TempDir(), "root")
	require.NoError(t, os.Mkdir(realRoot, 0700))
	parent := t.TempDir()
	root := filepath.Join(parent, "mirror")
	require.NoError(t, os.Symlink(realRoot, root))
	keyDir := filepath.Join(parent, "MIRROR", "signing")
	require.NoError(t, os.Mkdir(filepath.Join(realRoot, "signing"), 0700))
	info, err := os.Stat(keyDir)
	if os.IsNotExist(err) {
		t.Skip("filesystem does not alias directory case variants")
	}
	require.NoError(t, err)
	actual, err := os.Stat(filepath.Join(realRoot, "signing"))
	require.NoError(t, err)
	require.True(t, os.SameFile(info, actual))
	s, err := newServer(root, keyDir, "")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(keyDir, "private.json"), []byte("PRIVATE MARKER"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(realRoot, "root.json"), []byte("public metadata"), 0600))
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		w := httptest.NewRecorder()
		s.router().ServeHTTP(w, httptest.NewRequest(method, "/signing/private.json", nil))
		require.Equal(t, http.StatusForbidden, w.Code)
		require.NotContains(t, w.Body.String(), "MARKER")
	}
	w := httptest.NewRecorder()
	s.router().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/root.json", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "public metadata", w.Body.String())
}

func TestStaticServerUnicodePrivateRoot(t *testing.T) {
	for _, layout := range []string{"root", "ancestor", "combining-mark-order"} {
		t.Run(layout, func(t *testing.T) {
			parent := t.TempDir()
			private := filepath.Join(parent, "caf\u00e9")
			keyDir := filepath.Join(parent, "cafe\u0301")
			if layout == "combining-mark-order" {
				private = filepath.Join(parent, "\u1fb4")
				keyDir = filepath.Join(parent, "\u03b1\u0345\u0301")
			}
			require.NoError(t, os.Mkdir(private, 0700))
			info, err := os.Stat(keyDir)
			if os.IsNotExist(err) {
				t.Skip("filesystem does not alias NFC and NFD directory names")
			}
			require.NoError(t, err)
			actual, err := os.Stat(private)
			require.NoError(t, err)
			require.True(t, os.SameFile(info, actual))
			root := private
			if layout == "ancestor" {
				root = filepath.Join(private, "mirror")
				require.NoError(t, os.Mkdir(root, 0700))
			}
			s, err := newServer(root, keyDir, "")
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(root, "private.json"), []byte("PRIVATE MARKER"), 0600))
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				w := httptest.NewRecorder()
				s.router().ServeHTTP(w, httptest.NewRequest(method, "/private.json", nil))
				require.Equal(t, http.StatusForbidden, w.Code)
				require.NotContains(t, w.Body.String(), "MARKER")
			}
		})
	}
}
