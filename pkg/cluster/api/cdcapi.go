// Copyright 2022 PingCAP, Inc.
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

package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pingcap/tiup/pkg/utils"
)

type CDCOpenAPIClient struct {
	addrs      []string
	tlsEnabled bool
	client     *utils.HTTPClient
}

func NewCDCOpenAPIClient(addrs []string, timeout time.Duration, tlsConfig *tls.Config) *CDCOpenAPIClient {
	enableTLS := false
	if tlsConfig != nil {
		enableTLS = true
	}

	return &CDCOpenAPIClient{
		addrs:      addrs,
		tlsEnabled: enableTLS,
		client:     utils.NewHTTPClient(timeout, tlsConfig),
	}
}

func (c *CDCOpenAPIClient) DrainCapture(target string) error {
	api := "api/v1/captures/drain"

	request := &struct {
	}{}

	body, err := json.Marshal(request)
	if err != nil {
		return err
	}

	bytes, err := c.client.PUT(context.Background(), api, body)
	if err != nil {
		return err
	}

	var resp struct{}
	err = json.Unmarshal(bytes, resp)
	if err != nil {
		return err
	}

	return nil
}

func (c *CDCOpenAPIClient) ResignOwner() error {
	api := "api/v1/owner/resign"
	return nil
}

// GetURL builds the the client URL of DMClient
func (c *CDCOpenAPIClient) GetURL(addr string) string {
	httpPrefix := "http"
	if c.tlsEnabled {
		httpPrefix = "https"
	}
	return fmt.Sprintf("%s://%s", httpPrefix, addr)
}

func (c *CDCOpenAPIClient) getEndpoints(cmd string) (endpoints []string) {
	for _, addr := range c.addrs {
		endpoint := fmt.Sprintf("%s/%s", c.GetURL(addr), cmd)
		endpoints = append(endpoints, endpoint)
	}

	return endpoints
}

func (c *CDCOpenAPIClient) GetOwner() (interface{}, error) {
	return nil, nil
}

//var (
//	dmMembersURI = "apis/v1alpha1/members"
//
//	defaultRetryOpt = &utils.RetryOption{
//		Delay:   time.Second * 5,
//		Timeout: time.Second * 60,
//	}
//)

//func (dm *DMMasterClient) getMember(endpoints []string) (*dmpb.ListMemberResponse, error) {
//	resp := &dmpb.ListMemberResponse{}
//	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
//		body, err := dm.httpClient.Get(context.TODO(), endpoint)
//		if err != nil {
//			return body, err
//		}
//
//		err = jsonpb.Unmarshal(strings.NewReader(string(body)), resp)
//
//		if err != nil {
//			return body, err
//		}
//
//		if !resp.Result {
//			return body, errors.New("dm-master get members failed: " + resp.Msg)
//		}
//
//		return body, nil
//	})
//	return resp, err
//}
//
//// GetLeader gets leader of dm cluster
//func (dm *DMMasterClient) GetLeader(retryOpt *utils.RetryOption) (string, error) {
//	query := "?leader=true"
//	endpoints := dm.getEndpoints(dmMembersURI + query)
//
//	if retryOpt == nil {
//		retryOpt = defaultRetryOpt
//	}
//
//	var (
//		memberResp *dmpb.ListMemberResponse
//		err        error
//	)
//
//	if err := utils.Retry(func() error {
//		memberResp, err = dm.getMember(endpoints)
//		return err
//	}, *retryOpt); err != nil {
//		return "", err
//	}
//
//	leaderName := ""
//	for _, member := range memberResp.Members {
//		if leader := member.GetLeader(); leader != nil {
//			leaderName = leader.GetName()
//		}
//	}
//	return leaderName, nil
//}
