// Copyright 2021 PingCAP, Inc.
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

package proxy

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
)

var (
	httpProxy *http.Server
	tcpProxy  *TCPProxy
)

// MaybeStartProxy maybe starts an inner http/tcp proxies
func MaybeStartProxy(host string, port int, user string, usePass bool, identity string) error {
	if len(host) == 0 {
		return nil
	}

	sshProps, err := tui.ReadIdentityFileOrPassword(identity, usePass)
	if err != nil {
		return err
	}

	listenPort, err := utils.GetFreePort("127.0.0.1", 12345)
	if err != nil {
		return err
	}

	// TODO: Using environment variables to share data may not be a good idea
	os.Setenv("TIUP_INNER_HTTP_PROXY", fmt.Sprintf("http://127.0.0.1:%d", listenPort))
	httpProxy = &http.Server{
		Addr: fmt.Sprintf("127.0.0.1:%d", port),
		Handler: NewHTTPProxy(
			host, port, user,
			sshProps.Password,
			sshProps.IdentityFile,
			sshProps.IdentityFilePassphrase,
		),
	}

	go httpProxy.ListenAndServe()

	return nil
}

// MaMaybeStopProxy stops the http/tcp proxies if it has been started before
func MaybeStopProxy() {
	if httpProxy == nil {
		return
	}
	httpProxy.Shutdown(context.Background())
	os.Unsetenv("TIUP_INNER_HTTP_PROXY")
	httpProxy = nil
}
