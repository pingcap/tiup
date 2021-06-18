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
	"fmt"
	"net/http"
	"os"

	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
)

// MaybeStartProxy maybe starts an inner http proxy
func MaybeStartProxy(gOpt operator.Options) (*http.Server, error) {
	if len(gOpt.SSHProxyHost) == 0 {
		return nil, nil
	}

	sshProps, err := tui.ReadIdentityFileOrPassword(gOpt.SSHProxyIdentity, gOpt.SSHProxyUsePassword)
	if err != nil {
		return nil, err
	}

	port, err := utils.GetFreePort("127.0.0.1", 12345)
	if err != nil {
		return nil, err
	}

	// TODO: Using environment variables to share data may not be a good idea
	os.Setenv("TIUP_INNER_HTTP_PROXY", fmt.Sprintf("http://127.0.0.1:%d", port))
	server := &http.Server{
		Addr: fmt.Sprintf("127.0.0.1:%d", port),
		Handler: NewHTTPProxy(
			gOpt.SSHProxyHost,
			gOpt.SSHProxyPort,
			gOpt.SSHProxyUser,
			sshProps.Password,
			sshProps.IdentityFile,
			sshProps.IdentityFilePassphrase,
		),
	}

	go server.ListenAndServe()

	return server, nil
}
