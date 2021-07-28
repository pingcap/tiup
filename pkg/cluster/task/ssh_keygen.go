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
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/crypto/rand"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/crypto/ssh"
)

// SSHKeyGen is used to generate SSH key
type SSHKeyGen struct {
	keypath string
}

// Execute implements the Task interface
func (s *SSHKeyGen) Execute(ctx context.Context) error {
	ctxt.GetInner(ctx).Ev.PublishTaskProgress(s, "Generate SSH keys")

	savePrivateFileTo := s.keypath
	savePublicFileTo := s.keypath + ".pub"

	// Skip ssh key generate
	if utils.IsExist(savePrivateFileTo) && utils.IsExist(savePublicFileTo) {
		ctxt.GetInner(ctx).PublicKeyPath = savePublicFileTo
		ctxt.GetInner(ctx).PrivateKeyPath = savePrivateFileTo
		return nil
	}

	bitSize := 4096

	ctxt.GetInner(ctx).Ev.PublishTaskProgress(s, "Generate private key")
	privateKey, err := s.generatePrivateKey(bitSize)
	if err != nil {
		return errors.Trace(err)
	}

	ctxt.GetInner(ctx).Ev.PublishTaskProgress(s, "Generate public key")
	publicKeyBytes, err := s.generatePublicKey(&privateKey.PublicKey)
	if err != nil {
		return errors.Trace(err)
	}

	privateKeyBytes := s.encodePrivateKeyToPEM(privateKey)

	ctxt.GetInner(ctx).Ev.PublishTaskProgress(s, "Persist keys")
	err = s.writeKeyToFile(privateKeyBytes, savePrivateFileTo)
	if err != nil {
		return errors.Trace(err)
	}

	err = s.writeKeyToFile(publicKeyBytes, savePublicFileTo)
	if err != nil {
		return errors.Trace(err)
	}

	ctxt.GetInner(ctx).PublicKeyPath = savePublicFileTo
	ctxt.GetInner(ctx).PrivateKeyPath = savePrivateFileTo
	return nil
}

// generatePrivateKey creates a RSA Private Key of specified byte size
func (s *SSHKeyGen) generatePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, err
	}

	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

// encodePrivateKeyToPEM encodes Private Key from RSA to PEM format
func (s *SSHKeyGen) encodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	// Get ASN.1 DER format
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	// pem.Block
	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	// Private key in PEM format
	return pem.EncodeToMemory(&privBlock)
}

// generatePublicKey take a rsa.PublicKey and return bytes suitable for writing to .pub file
// returns in the format "ssh-rsa ..."
func (s *SSHKeyGen) generatePublicKey(privatekey *rsa.PublicKey) ([]byte, error) {
	publicRsaKey, err := ssh.NewPublicKey(privatekey)
	if err != nil {
		return nil, err
	}

	return ssh.MarshalAuthorizedKey(publicRsaKey), nil
}

// writePemToFile writes keys to a file
func (s *SSHKeyGen) writeKeyToFile(keyBytes []byte, saveFileTo string) error {
	if err := os.MkdirAll(filepath.Dir(saveFileTo), 0700); err != nil {
		return err
	}
	return os.WriteFile(saveFileTo, keyBytes, 0600)
}

// Rollback implements the Task interface
func (s *SSHKeyGen) Rollback(ctx context.Context) error {
	return os.Remove(s.keypath)
}

// String implements the fmt.Stringer interface
func (s *SSHKeyGen) String() string {
	return fmt.Sprintf("SSHKeyGen: path=%s", s.keypath)
}
