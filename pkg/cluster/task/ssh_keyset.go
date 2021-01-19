// Copyright 2020 PingCAP, Inc.
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

package task

import (
	"context"
	"fmt"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
)

// SSHKeySet is used to set the Context private/public key path
type SSHKeySet struct {
	privateKeyPath string
	publicKeyPath  string
}

// Execute implements the Task interface
func (s *SSHKeySet) Execute(ctx context.Context) error {
	ctxt.GetInner(ctx).PublicKeyPath = s.publicKeyPath
	ctxt.GetInner(ctx).PrivateKeyPath = s.privateKeyPath
	return nil
}

// Rollback implements the Task interface
func (s *SSHKeySet) Rollback(ctx context.Context) error {
	ctxt.GetInner(ctx).PublicKeyPath = ""
	ctxt.GetInner(ctx).PrivateKeyPath = ""
	return nil
}

// String implements the fmt.Stringer interface
func (s *SSHKeySet) String() string {
	return fmt.Sprintf("SSHKeySet: privateKey=%s, publicKey=%s", s.privateKeyPath, s.publicKeyPath)
}
