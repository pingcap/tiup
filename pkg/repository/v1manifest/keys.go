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
	"crypto/sha256"
	"fmt"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/crypto"
)

// ShortKeyIDLength is the number of bytes used for filenames
const ShortKeyIDLength = 16

// ErrorNotPrivateKey indicate that it need a private key, but the supplied is not.
var ErrorNotPrivateKey = errors.New("not a private key")

// NewKeyInfo make KeyInfo from private key, public key should be load from json
func NewKeyInfo(privKey []byte) *KeyInfo {
	// TODO: support other key type and scheme
	return &KeyInfo{
		Type:   crypto.KeyTypeRSA,
		Scheme: crypto.KeySchemeRSASSAPSSSHA256,
		Value: map[string]string{
			"private": string(privKey),
		},
	}
}

// GenKeyInfo generate a new private KeyInfo
func GenKeyInfo() (*KeyInfo, error) {
	// TODO: support other key type and scheme
	priv, err := crypto.NewKeyPair(crypto.KeyTypeRSA, crypto.KeySchemeRSASSAPSSSHA256)
	if err != nil {
		return nil, err
	}
	bytes, err := priv.Serialize()
	if err != nil {
		return nil, err
	}
	return NewKeyInfo(bytes), nil
}

// ID returns the hash id of the key
func (ki *KeyInfo) ID() (string, error) {
	// Make sure private key and correspond public key has the same id
	pk, err := ki.publicKey()
	if err != nil {
		return "", err
	}
	data, err := pk.Serialize()
	if err != nil {
		return "", err
	}
	value := map[string]string{
		"public": string(data),
	}

	payload, err := cjson.Marshal(KeyInfo{
		Type:   ki.Type,
		Scheme: ki.Scheme,
		Value:  value,
	})
	if err != nil {
		// XXX: maybe we can assume that the error should always be nil since the KeyInfo struct is valid
		return "", err
	}
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("%x", sum), nil
}

// IsPrivate detect if this is a private key
func (ki *KeyInfo) IsPrivate() bool {
	return len(ki.Value["private"]) > 0
}

// Signature sign a signature with the key for payload
func (ki *KeyInfo) Signature(payload []byte) (string, error) {
	pk, err := ki.privateKey()
	if err != nil {
		return "", err
	}
	return pk.Signature(payload)
}

// SignManifest wrap Signature with the param manifest
func (ki *KeyInfo) SignManifest(m ValidManifest) (string, error) {
	payload, err := cjson.Marshal(m)
	if err != nil {
		return "", errors.Annotate(err, "marshal for signature")
	}
	return ki.Signature(payload)
}

// Verify check the signature is right
func (ki *KeyInfo) Verify(payload []byte, sig string) error {
	pk, err := ki.publicKey()
	if err != nil {
		return err
	}
	return pk.VerifySignature(payload, sig)
}

// Public returns the public keyInfo
func (ki *KeyInfo) Public() (*KeyInfo, error) {
	pk, err := ki.publicKey()
	if err != nil {
		return nil, err
	}
	bytes, err := pk.Serialize()
	if err != nil {
		return nil, err
	}
	return &KeyInfo{
		Type:   pk.Type(),
		Scheme: pk.Scheme(),
		Value: map[string]string{
			"public": string(bytes),
		},
	}, nil
}

// publicKey returns PubKey
func (ki *KeyInfo) publicKey() (crypto.PubKey, error) {
	if ki.IsPrivate() {
		priv, err := ki.privateKey()
		if err != nil {
			return nil, err
		}
		return priv.Public(), nil
	}
	return crypto.NewPubKey(ki.Type, ki.Scheme, []byte(ki.Value["public"]))
}

// privateKey returns PrivKey
func (ki *KeyInfo) privateKey() (crypto.PrivKey, error) {
	if !ki.IsPrivate() {
		return nil, ErrorNotPrivateKey
	}

	return crypto.NewPrivKey(ki.Type, ki.Scheme, []byte(ki.Value["private"]))
}
