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
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/appleboy/easyssh-proxy"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/utils"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

type TCPProxy struct {
	l         sync.RWMutex
	cli       *ssh.Client
	config    *easyssh.MakeConfig
	endpoints []string
	closeC    chan struct{}
}

func NewTCPProxy(host string, port int, user, password, keyFile, passphrase string, endpoints []string) *TCPProxy {
	p := &TCPProxy{
		config: &easyssh.MakeConfig{
			Server:  host,
			Port:    strconv.Itoa(port),
			User:    user,
			Timeout: 10 * time.Second,
		},
		endpoints: endpoints,
	}

	if len(keyFile) > 0 {
		p.config.KeyPath = keyFile
		p.config.Passphrase = passphrase
	} else if len(password) > 0 {
		p.config.Password = password
	}

	port, err := utils.GetFreePort("127.0.0.1", 22345)
	if err != nil {
		return nil
	}

	localListener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		log.Fatalf("net.Listen failed: %v", err)
	}

	go func() {
		for {
			select {
			case <-p.closeC:
				return
			default:
				localConn, err := localListener.Accept()
				if err != nil {
					log.Fatalf("listen.Accept failed: %v", err)
				}
				go p.forward(localConn)
			}
		}
	}()

	return p
}

func (p *TCPProxy) getConn() (*ssh.Client, error) {
	p.l.RLock()
	cli := p.cli
	p.l.RUnlock()

	// reuse the old client if dial success
	if cli == nil {
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

func (p *TCPProxy) forward(localConn net.Conn) {
	cli, err := p.getConn()
	if err != nil {
		zap.L().Error("Failed to get ssh client", zap.String("error", err.Error()))
		return
	}

	var remoteConn net.Conn
OUTER_LOOP:
	for _, endpoint := range p.endpoints {
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
