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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStaticServerPrivateRequestHeaders(t *testing.T) {
	modified := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	for _, keyName := range []string{"keys", "private-signing"} {
		t.Run(keyName, func(t *testing.T) {
			root := t.TempDir()
			keyDir := filepath.Join(root, keyName)
			require.NoError(t, os.Mkdir(keyDir, 0700))
			privateFile := filepath.Join(keyDir, "private.json")
			var upstreamRequests atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamRequests.Add(1)
				_, _ = w.Write([]byte("UPSTREAM MARKER"))
			}))
			defer upstream.Close()
			s, err := newServer(root, keyDir, upstream.URL)
			require.NoError(t, err)
			// Install inert key markers only after signing-key initialization.
			require.NoError(t, os.WriteFile(privateFile, []byte("PRIVATE TEST MARKER"), 0600))
			require.NoError(t, os.Chtimes(privateFile, modified, modified))
			server := httptest.NewServer(s.router())
			defer server.Close()
			client := server.Client()
			client.Timeout = 5 * time.Second
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				for _, file := range []string{"private.json", "missing.json"} {
					for _, test := range []struct {
						name    string
						headers map[string]string
					}{
						{"range", map[string]string{"Range": "bytes=0-6"}},
						{"if-modified-since", map[string]string{"If-Modified-Since": modified.Format(http.TimeFormat)}},
						{"if-unmodified-since", map[string]string{"If-Unmodified-Since": modified.Add(-time.Hour).Format(http.TimeFormat)}},
						{"if-none-match", map[string]string{"If-None-Match": "*"}},
						{"if-match", map[string]string{"If-Match": `"not-the-private-file"`}},
						{"if-range", map[string]string{"Range": "bytes=0-6", "If-Range": modified.Format(http.TimeFormat)}},
					} {
						t.Run(method+"/"+file+"/"+test.name, func(t *testing.T) {
							r, err := http.NewRequest(method, server.URL+"/"+keyName+"/"+file, nil)
							require.NoError(t, err)
							for key, value := range test.headers {
								r.Header.Set(key, value)
							}
							resp, err := client.Do(r)
							require.NoError(t, err)
							defer resp.Body.Close()
							body, err := io.ReadAll(resp.Body)
							require.NoError(t, err)
							require.Equal(t, http.StatusForbidden, resp.StatusCode)
							require.NotContains(t, string(body), "MARKER")
							if method == http.MethodHead {
								require.Empty(t, body)
							} else {
								// A short Range could disclose only part of the marker.
								require.Equal(t, "403 Forbidden\n", string(body))
							}
							require.Empty(t, resp.Header.Get("Content-Range"))
							require.Empty(t, resp.Header.Get("Last-Modified"))
							require.Empty(t, resp.Header.Get("ETag"))
							require.Zero(t, upstreamRequests.Load())
						})
					}
				}
			}
		})
	}
}

func TestStaticServerRejectedEntries(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing")
	require.NoError(t, os.Symlink(missing, filepath.Join(root, "dangling-file")))
	require.NoError(t, os.Symlink(missing, filepath.Join(root, "dangling-directory")))
	for _, name := range []string{"dangling-file", "dangling-directory"} {
		info, err := os.Lstat(filepath.Join(root, name))
		require.NoError(t, err)
		require.NotZero(t, info.Mode()&os.ModeSymlink)
		_, err = os.Stat(filepath.Join(root, name))
		require.True(t, os.IsNotExist(err), "fixture must be a dangling symlink")
	}
	var upstreamRequests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamRequests.Add(1)
		_, _ = w.Write([]byte("UPSTREAM MARKER"))
	}))
	defer upstream.Close()
	s, err := newServer(root, "", upstream.URL)
	require.NoError(t, err)
	server := httptest.NewServer(s.router())
	defer server.Close()
	client := server.Client()
	client.Timeout = 5 * time.Second
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, test := range []struct {
			name   string
			path   string
			status int
		}{
			{"dangling-file", "/dangling-file", http.StatusForbidden},
			{"dangling-directory", "/dangling-directory/file.json", http.StatusForbidden},
			{"nul-file", "/public%00file.json", http.StatusInternalServerError},
			{"nul-directory", "/public%00directory/file.json", http.StatusInternalServerError},
		} {
			t.Run(method+"/"+test.name, func(t *testing.T) {
				r, err := http.NewRequest(method, server.URL+test.path, nil)
				require.NoError(t, err)
				resp, err := client.Do(r)
				require.NoError(t, err)
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Equal(t, test.status, resp.StatusCode)
				if method == http.MethodGet {
					require.Equal(t, fmt.Sprintf("%d %s\n", test.status, http.StatusText(test.status)), string(body))
				} else {
					require.Empty(t, body)
				}
				require.NotContains(t, string(body), root)
				require.NotContains(t, string(body), missing)
				require.NotContains(t, string(body), "MARKER")
				require.Zero(t, upstreamRequests.Load())
			})
		}
	}
}

func TestStaticServerPublicConditionalHTTP(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "root.json")
	modified := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	require.NoError(t, os.WriteFile(file, []byte("public metadata"), 0600))
	require.NoError(t, os.Chtimes(file, modified, modified))
	s, err := newServer(root, "", "")
	require.NoError(t, err)
	server := httptest.NewServer(s.router())
	defer server.Close()
	client := server.Client()
	client.Timeout = 5 * time.Second
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, test := range []struct {
			name   string
			since  time.Time
			status int
		}{
			{"unchanged", modified, http.StatusNotModified},
			{"stale", modified.Add(-time.Hour), http.StatusOK},
		} {
			t.Run(method+"/"+test.name, func(t *testing.T) {
				r, err := http.NewRequest(method, server.URL+"/root.json", nil)
				require.NoError(t, err)
				r.Header.Set("If-Modified-Since", test.since.Format(http.TimeFormat))
				resp, err := client.Do(r)
				require.NoError(t, err)
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Equal(t, test.status, resp.StatusCode)
				require.Equal(t, modified.Format(http.TimeFormat), resp.Header.Get("Last-Modified"))
				if method == http.MethodGet && test.status == http.StatusOK {
					require.Equal(t, "public metadata", string(body))
				} else {
					require.Empty(t, body)
				}
			})
		}
	}
}

func TestStaticServerUpstreamRequestPreservation(t *testing.T) {
	type upstreamRequest struct {
		method, path, query, uri, rangeHeader, modifiedSince string
	}
	requests := make(chan upstreamRequest, 1)
	var upstreamRequests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamRequests.Add(1)
		// Do not block server shutdown if a regression sends extra requests.
		// The counter below independently requires exactly one upstream request.
		select {
		case requests <- upstreamRequest{r.Method, r.URL.Path, r.URL.RawQuery, r.RequestURI, r.Header.Get("Range"), r.Header.Get("If-Modified-Since")}:
		default:
		}
		w.Header().Set("Content-Range", "bytes 0-5/14")
		w.Header().Set("Content-Length", "6")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("public"))
	}))
	defer upstream.Close()
	s, err := newServer(t.TempDir(), "", upstream.URL+"/mirror?source=default")
	require.NoError(t, err)
	server := httptest.NewServer(s.router())
	defer server.Close()
	client := server.Client()
	client.Timeout = 5 * time.Second
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			before := upstreamRequests.Load()
			query := "version=v1%2Bbuild&tag=a&tag=b&empty=&path=a%2Fb"
			r, err := http.NewRequest(method, server.URL+"/packages/a%20b%2B.tar.gz?"+query, nil)
			require.NoError(t, err)
			r.Header.Set("Range", "bytes=0-5")
			r.Header.Set("If-Modified-Since", "Tue, 02 Jan 2024 03:04:05 GMT")
			resp, err := client.Do(r)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, http.StatusPartialContent, resp.StatusCode)
			require.Equal(t, "bytes 0-5/14", resp.Header.Get("Content-Range"))
			require.Equal(t, "6", resp.Header.Get("Content-Length"))
			if method == http.MethodGet {
				require.Equal(t, "public", string(body))
			} else {
				require.Empty(t, body)
			}
			require.Equal(t, before+1, upstreamRequests.Load())
			select {
			case request := <-requests:
				require.Equal(t, upstreamRequest{
					method:        method,
					path:          "/mirror/packages/a b+.tar.gz",
					query:         "source=default&" + query,
					uri:           "/mirror/packages/a%20b%2B.tar.gz?source=default&" + query,
					rangeHeader:   "bytes=0-5",
					modifiedSince: "Tue, 02 Jan 2024 03:04:05 GMT",
				}, request)
			default:
				t.Fatal("upstream received no request")
			}
		})
	}
}
