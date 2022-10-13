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

package v1manifest

import (
	"fmt"
	"sync"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/crypto"
)

// KeyStore tracks roles, keys, etc. and verifies signatures against this metadata. (map[string]roleKeys)
type KeyStore struct {
	sync.Map
}

type roleKeys struct {
	threshold uint
	expiry    string
	// key id -> public key (map[string]crypto.PubKey)
	keys *sync.Map
}

// NewKeyStore return a KeyStore
func NewKeyStore() *KeyStore {
	return &KeyStore{}
}

// AddKeys clears all keys for role, then adds all supplied keys and stores the threshold value.
func (s *KeyStore) AddKeys(role string, threshold uint, expiry string, keys map[string]*KeyInfo) error {
	if threshold == 0 {
		return errors.Errorf("invalid threshold (0)")
	}

	rk := roleKeys{threshold: threshold, expiry: expiry, keys: &sync.Map{}}

	for id, info := range keys {
		pub, err := info.publicKey()
		if err != nil {
			return err
		}

		rk.keys.Store(id, pub)
	}
	s.Store(role, rk)

	return nil
}

func newSignatureError(fname string, err error) *SignatureError {
	return &SignatureError{
		fname: fname,
		err:   err,
	}
}

// IsSignatureError check if the err is SignatureError.
func IsSignatureError(err error) bool {
	_, ok := err.(*SignatureError)
	return ok
}

// SignatureError the signature of a file is incorrect.
type SignatureError struct {
	fname string
	err   error
}

func (s *SignatureError) Error() string {
	return fmt.Sprintf("invalid signature for file %s: %s", s.fname, s.err.Error())
}

// transitionRoot checks that signed is verified by signatures using newThreshold, and if so, updates the keys for the root
// role in the key store.
func (s *KeyStore) transitionRoot(signed []byte, newThreshold uint, expiry string, signatures []Signature, newKeys map[string]*KeyInfo) error {
	if s == nil {
		return nil
	}

	oldKeys, hasOldKeys := s.Load(ManifestTypeRoot)

	err := s.AddKeys(ManifestTypeRoot, newThreshold, expiry, newKeys)
	if err != nil {
		return err
	}

	err = s.verifySignature(signed, ManifestTypeRoot, signatures, ManifestFilenameRoot)
	if err != nil {
		// Restore the old root keys.
		if hasOldKeys {
			s.Store(ManifestTypeRoot, oldKeys)
		}
		return err
	}

	return nil
}

// verifySignature verifies all supplied signatures are correct for signed. Also, that there are at least threshold signatures,
// and that they all belong to the correct role and are correct for signed. It is permissible for signature keys to not
// exist (they will be ignored, and not count towards the threshold) but not for a signature to be incorrect.
func (s *KeyStore) verifySignature(signed []byte, role string, signatures []Signature, filename string) error {
	if s == nil {
		return nil
	}

	// Check for duplicate signatures.
	has := make(map[string]struct{})
	for _, sig := range signatures {
		if _, ok := has[sig.KeyID]; ok {
			return newSignatureError(filename, errors.Errorf("signature section of %s contains duplicate signatures", filename))
		}
		has[sig.KeyID] = struct{}{}
	}

	ks, ok := s.Load(role)
	if !ok {
		return errors.Errorf("Unknown role %s", role)
	}
	keys := ks.(roleKeys)

	var validSigs uint
	for _, sig := range signatures {
		key, ok := keys.keys.Load(sig.KeyID)
		if !ok {
			continue
		}
		err := key.(crypto.PubKey).VerifySignature(signed, sig.Sig)
		if err != nil {
			return newSignatureError(filename, err)
		}
		validSigs++
	}

	// We may need to verify the root manifest with old keys. Once the most up to date root is found and verified, then
	// the keys used to do so should be checked for expiry.
	if role != ManifestTypeRoot {
		if err := CheckExpiry(filename, keys.expiry); err != nil {
			return err
		}
	}
	if validSigs < keys.threshold {
		return newSignatureError(filename, errors.Errorf("not enough signatures (%v) for threshold %v in %s", validSigs, keys.threshold, filename))
	}

	return nil
}
