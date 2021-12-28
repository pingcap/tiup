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

package operator

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/spec"
)

// UpdatePDMember is used to update pd cluster member
type UpdatePDMember struct {
	cluster   string
	tlsCfg    *tls.Config
	metadata  spec.Metadata
	enableTLS bool
}

// SetPDMember set the member of pd-etcd
func SetPDMember(ctx context.Context, clusterName string, enable bool, tlsCfg *tls.Config, meta spec.Metadata) error {
	u := &UpdatePDMember{
		cluster:   clusterName,
		tlsCfg:    tlsCfg,
		metadata:  meta,
		enableTLS: enable,
	}

	return u.Execute(ctx)
}

// Execute implements the Task interface
func (u *UpdatePDMember) Execute(ctx context.Context) error {
	// connection etcd
	etcdClient, err := u.metadata.GetTopology().(*spec.Specification).GetEtcdClient(u.tlsCfg)
	if err != nil {
		return err
	}
	// etcd client defaults to wait forever
	// if all pd were down, don't hang forever
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// txn := etcdClient.Txn(ctx)

	memberList, err := etcdClient.MemberList(ctx)
	if err != nil {
		return err
	}

	for _, member := range memberList.Members {
		_, err := etcdClient.MemberUpdate(ctx, member.GetID(), u.updatePeerURLs(member.PeerURLs))
		if err != nil {
			return err
		}
	}

	// get member list after update
	memberList, err = etcdClient.MemberList(ctx)
	if err != nil {
		return err
	}
	for _, member := range memberList.Members {
		fmt.Printf("\tUpdate %s peerURLs: %v\n", member.Name, member.PeerURLs)
	}

	return nil
}

// updatePeerURLs http->https or https->http
func (u *UpdatePDMember) updatePeerURLs(peerURLs []string) []string {
	newPeerURLs := []string{}

	for _, url := range peerURLs {
		if u.enableTLS {
			newPeerURLs = append(newPeerURLs, strings.Replace(url, "http://", "https://", 1))
		} else {
			newPeerURLs = append(newPeerURLs, strings.Replace(url, "https://", "http://", 1))
		}
	}

	return newPeerURLs
}
