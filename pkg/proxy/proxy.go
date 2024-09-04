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
	"sync/atomic"

	"github.com/fatih/color"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
)

var (
	httpProxy *http.Server
	tcpProxy  atomic.Value
)

// MaybeStartProxy maybe starts an inner http/tcp proxies
func MaybeStartProxy(
	host string,
	port int,
	user string,
	usePass bool,
	identity string,
	logger *logprinter.Logger,
) error {
	if len(host) == 0 {
		return nil
	}

	sshProps, err := tui.ReadIdentityFileOrPassword(identity, usePass)
	if err != nil {
		return err
	}

	httpPort := utils.MustGetFreePort("127.0.0.1", 12345, 0)
	addr := fmt.Sprintf("127.0.0.1:%d", httpPort)

	// TODO: Using environment variables to share data may not be a good idea
	os.Setenv("TIUP_INNER_HTTP_PROXY", "http://"+addr)
	httpProxy = &http.Server{
		Addr: addr,
		Handler: NewHTTPProxy(
			host, port, user,
			sshProps.Password,
			sshProps.IdentityFile,
			sshProps.IdentityFilePassphrase,
			logger,
		),
	}

	logger.Infof(color.HiGreenString("Start HTTP inner proxy %s", httpProxy.Addr))
	go func() {
		if err := httpProxy.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Failed to listen HTTP proxy: %v", err)
		}
	}()

	p := NewTCPProxy(
		host, port, user,
		sshProps.Password,
		sshProps.IdentityFile,
		sshProps.IdentityFilePassphrase,
		logger,
	)
	tcpProxy.Store(p)

	logger.Infof(color.HiGreenString("Start TCP inner proxy %s", p.endpoint))

	return nil
}

// MaybeStopProxy stops the http/tcp proxies if it has been started before
func MaybeStopProxy() {
	if httpProxy != nil {
		_ = httpProxy.Shutdown(context.Background())
		os.Unsetenv("TIUP_INNER_HTTP_PROXY")
	}
	if p := tcpProxy.Load(); p != nil {
		_ = p.(*TCPProxy).Stop()
	}
}

// GetTCPProxy returns the tcp proxy
func GetTCPProxy() *TCPProxy {
	p := tcpProxy.Load()
	if p != nil {
		return p.(*TCPProxy)
	}
	return nil
}
