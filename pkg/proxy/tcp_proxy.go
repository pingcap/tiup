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
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/appleboy/easyssh-proxy"
	perrs "github.com/pingcap/errors"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/utils"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

// TCPProxy represents a simple TCP proxy
// unlike HTTP proxies, TCP proxies are point-to-point
type TCPProxy struct {
	l        sync.RWMutex
	listener net.Listener
	cli      *ssh.Client
	config   *easyssh.MakeConfig
	closed   int32
	endpoint string
	logger   *logprinter.Logger
}

// NewTCPProxy starts a 1to1 TCP proxy
func NewTCPProxy(
	host string,
	port int,
	user, password, keyFile, passphrase string,
	logger *logprinter.Logger,
) *TCPProxy {
	p := &TCPProxy{
		config: &easyssh.MakeConfig{
			Server:  host,
			Port:    strconv.Itoa(port),
			User:    user,
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}

	if len(keyFile) > 0 {
		p.config.KeyPath = keyFile
		p.config.Passphrase = passphrase
	} else if len(password) > 0 {
		p.config.Password = password
	}

	port = utils.MustGetFreePort("127.0.0.1", 22345, 0)
	p.endpoint = fmt.Sprintf("127.0.0.1:%d", port)

	listener, err := net.Listen("tcp", p.endpoint)
	if err != nil {
		logger.Errorf("net.Listen error: %v", err)
		return nil
	}
	p.listener = listener

	return p
}

// GetEndpoints returns the endpoint list
func (p *TCPProxy) GetEndpoints() []string {
	return []string{p.endpoint}
}

// Stop stops the tcp proxy
func (p *TCPProxy) Stop() error {
	atomic.StoreInt32(&p.closed, 1)
	return p.listener.Close()
}

// Run runs proxy all traffic to upstream
func (p *TCPProxy) Run(upstream []string) chan struct{} {
	closeC := make(chan struct{})
	go func() {
	FOR_LOOP:
		for {
			select {
			case <-closeC:
				return
			default:
				localConn, err := p.listener.Accept()
				if err != nil {
					if atomic.LoadInt32(&p.closed) == 1 {
						break FOR_LOOP
					}
					p.logger.Errorf("tcp proxy accept error: %v", err)
					continue FOR_LOOP
				}
				go p.forward(localConn, upstream)
			}
		}
	}()
	return closeC
}

// Close closes a proxy
func (p *TCPProxy) Close(c chan struct{}) {
	close(c)
}

func (p *TCPProxy) getConn() (*ssh.Client, error) {
	p.l.RLock()
	cli := p.cli
	p.l.RUnlock()

	// reuse the old client if dial success
	if cli != nil {
		return cli, nil
	}

	// create a new ssh client
	_, cli, err := p.config.Connect()
	if err != nil {
		return nil, perrs.Annotate(err, "connect to ssh proxy")
	}

	p.l.Lock()
	p.cli = cli
	p.l.Unlock()

	return cli, nil
}

func (p *TCPProxy) forward(localConn io.ReadWriter, endpoints []string) {
	cli, err := p.getConn()
	if err != nil {
		zap.L().Error("Failed to get ssh client", zap.String("error", err.Error()))
		return
	}

	var remoteConn net.Conn
OUTER_LOOP:
	for _, endpoint := range endpoints {
		endpoint := endpoint
		errC := make(chan error, 1)
		go func() {
			var err error
			remoteConn, err = cli.Dial("tcp", endpoint)
			if err != nil {
				zap.L().Error("Failed to connect endpoint", zap.String("error", err.Error()))
			}

			errC <- err
		}()

		select {
		case err := <-errC:
			if err == nil {
				break OUTER_LOOP
			}
		case <-time.After(5 * time.Second):
			zap.L().Debug("Connect to endpoint timeout, retry the next endpoint", zap.String("endpoint", endpoint))
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, err = io.Copy(remoteConn, localConn)
		if err != nil {
			zap.L().Error("Failed to copy from local to remote", zap.String("error", err.Error()))
		}
	}()

	go func() {
		defer wg.Done()
		_, err = io.Copy(localConn, remoteConn)
		if err != nil {
			zap.L().Error("Failed to copy from remote to local", zap.String("error", err.Error()))
		}
	}()
	wg.Wait()
}
