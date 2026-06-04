// Copyright 2024 PingCAP, Inc.
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
	"net/http"
	"testing"
)

// transportProxyFor returns the proxy URL string that makeTransport would use
// for rawurl ("" means a direct connection / no proxy).
func transportProxyFor(t *testing.T, rawurl string) string {
	t.Helper()
	tr := makeTransport(nil)
	if tr.Proxy == nil {
		t.Fatal("transport proxy func is not set")
	}
	req, err := http.NewRequest(http.MethodGet, rawurl, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := tr.Proxy(req)
	if err != nil {
		t.Fatal(err)
	}
	if u == nil {
		return ""
	}
	return u.String()
}

// TestMakeTransportHonorsNoProxy verifies that a host listed in NO_PROXY (e.g.
// an internal TiKV status endpoint scraped for the leader count) is reached
// directly even when HTTP(S)_PROXY is set, while other hosts use the proxy.
func TestMakeTransportHonorsNoProxy(t *testing.T) {
	t.Setenv("TIUP_INNER_HTTP_PROXY", "")
	t.Setenv("HTTP_PROXY", "http://proxy.example.com:3128")
	t.Setenv("HTTPS_PROXY", "http://proxy.example.com:3128")
	t.Setenv("NO_PROXY", "10.0.0.0/8,.internal.example.com")

	if got := transportProxyFor(t, "http://tikv-1.internal.example.com:20180/metrics"); got != "" {
		t.Errorf("host in NO_PROXY must bypass the proxy, got %q", got)
	}
	if got := transportProxyFor(t, "https://tiup-mirrors.pingcap.com/timestamp.json"); got != "http://proxy.example.com:3128" {
		t.Errorf("external host must use the proxy, got %q", got)
	}
}

// TestMakeTransportInnerProxyRespectsNoProxy verifies the TIUP_INNER_HTTP_PROXY
// override still applies to external hosts but also respects NO_PROXY.
func TestMakeTransportInnerProxyRespectsNoProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", ".internal.example.com")
	t.Setenv("TIUP_INNER_HTTP_PROXY", "http://inner.example.com:3128")

	if got := transportProxyFor(t, "https://tiup-mirrors.pingcap.com/timestamp.json"); got != "http://inner.example.com:3128" {
		t.Errorf("external host must use the inner proxy, got %q", got)
	}
	if got := transportProxyFor(t, "http://tikv-1.internal.example.com:20180/metrics"); got != "" {
		t.Errorf("host in NO_PROXY must bypass the inner proxy, got %q", got)
	}
}
