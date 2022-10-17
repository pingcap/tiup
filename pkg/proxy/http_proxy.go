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
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/appleboy/easyssh-proxy"
	perrs "github.com/pingcap/errors"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

// HTTPProxy stands for a http proxy based on SSH connection
type HTTPProxy struct {
	cli    *ssh.Client
	config *easyssh.MakeConfig
	l      sync.RWMutex
	tr     *http.Transport
	logger *logprinter.Logger
}

// NewHTTPProxy creates and initializes a new http proxy
func NewHTTPProxy(host string,
	port int,
	user, password, keyFile, passphrase string,
	logger *logprinter.Logger,
) *HTTPProxy {
	p := &HTTPProxy{
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

	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		p.l.RLock()
		cli := p.cli
		p.l.RUnlock()

		// reuse the old client if dial success
		if cli != nil {
			c, err := cli.Dial(network, addr)
			if err == nil {
				return c, nil
			}
		}

		// create a new ssh client
		// timeout is implemented inside easyssh, don't need to repeat the implementation
		_, cli, err := p.config.Connect()
		if err != nil {
			return nil, perrs.Annotate(err, "connect to ssh proxy")
		}

		p.l.Lock()
		p.cli = cli
		p.l.Unlock()

		return cli.Dial(network, addr)
	}

	p.tr = &http.Transport{DialContext: dial}
	return p
}

// ServeHTTP implements http.Handler
func (s *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodConnect:
		s.connect(w, r)
	default:
		r.RequestURI = ""
		removeHopHeaders(r.Header)
		s.serve(w, r)
	}
}

// connect handles the CONNECT request
// Data flow:
//  1. Receive CONNECT request from the client
//  2. Dial the remote server(the one client want to conenct)
//  3. Send 200 OK to client if the connection is established
//  4. Exchange data between client and server
func (s *HTTPProxy) connect(w http.ResponseWriter, r *http.Request) {
	// Use Hijacker to get the underlying connection
	hij, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Server does not support Hijacker", http.StatusInternalServerError)
		return
	}

	// connect the remote client directly
	dst, err := s.tr.DialContext(context.Background(), "tcp", r.URL.Host)
	if err != nil {
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			zap.L().Debug("CONNECT roundtrip proxy timeout")
			return
		}
		zap.L().Debug("CONNECT roundtrip proxy", zap.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	src, _, err := hij.Hijack()
	if err != nil {
		zap.L().Debug("CONNECT hijack", zap.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer src.Close()

	// Once connected successfully, return OK
	_, _ = src.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))

	// Proxy is no need to know anything, just exchange data between the client
	// the the remote server.
	var wg sync.WaitGroup
	copyAndWait := func(dst, src net.Conn) {
		defer wg.Done()
		_, err := io.Copy(dst, src)
		if err != nil {
			zap.L().Error("CONNECT copy response", zap.Any("src", src), zap.Any("dst", dst))
		}
		if tcpConn, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = tcpConn.CloseWrite()
		}
	}

	wg.Add(2)
	go copyAndWait(dst, src) // client to remote
	go copyAndWait(src, dst) // remote to client
	wg.Wait()
}

// serve handles the original http request
// Data flow:
//  1. Receive request R1 from client
//  2. Re-post request R1 to remote server(the one client want to connect)
//  3. Receive response P1 from remote server
//  4. Send response P1 to client
func (s *HTTPProxy) serve(w http.ResponseWriter, r *http.Request) {
	// Client.Do is different from DefaultTransport.RoundTrip ...
	// Client.Do will change some part of request as a new request of the server.
	// The underlying RoundTrip never changes anything of the request.
	resp, err := s.tr.RoundTrip(r)
	if err != nil {
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			zap.L().Debug("PROXY roundtrip proxy timeout")
			return
		}
		zap.L().Debug("PROXY roundtrip proxy", zap.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// please prepare header first and write them
	copyHeaders(w, resp)
	w.WriteHeader(resp.StatusCode)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		zap.L().Error("PROXY copy response", zap.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	// If no Accept-Encoding header exists, Transport will add the headers it can accept
	// and would wrap the response body with the relevant reader.
	"Accept-Encoding",
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Proxy-Connection",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

// removeHopHeaders removes the hop-by-hop headers
func removeHopHeaders(h http.Header) {
	for _, k := range hopHeaders {
		h.Del(k)
	}
}

// copy and overwrite headers from r to w
func copyHeaders(w http.ResponseWriter, r *http.Response) {
	// copy headers
	dst, src := w.Header(), r.Header
	for k := range dst {
		dst.Del(k)
	}
	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}
