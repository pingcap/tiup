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

package crypto

import (
	"errors"
)

var (
	// ErrorKeyUninitialized will be present when key is used before Deserialize called
	ErrorKeyUninitialized = errors.New("key not initialized, call Deserialize first")
	// ErrorDeserializeKey means the key format is not valid
	ErrorDeserializeKey = errors.New("error on deserialize key, check if the key is valid")
	// ErrorUnsupportedKeyType means we don't supported this type of key
	ErrorUnsupportedKeyType = errors.New("provided key type not supported")
	// ErrorUnsupportedKeySchema means we don't support this schema
	ErrorUnsupportedKeySchema = errors.New("provided schema not supported")
)

const (
	// KeyTypeRSA represents the RSA type of keys
	KeyTypeRSA = "rsa"

	// KeySchemeRSASSAPSSSHA256 represents rsassa-pss-sha256 scheme
	KeySchemeRSASSAPSSSHA256 = "rsassa-pss-sha256"
)

// Serializable represents object that can be serialized and deserialized
type Serializable interface {
	// Translate the key to the format that can be stored
	Serialize() ([]byte, error)

	// Deserialize a key from data
	Deserialize([]byte) error
}

// PubKey is a public key available to TiUp
type PubKey interface {
	Serializable
	// Type returns the type of the key, e.g. RSA
	Type() string
	// Scheme returns the scheme of  signature algorithm, e.g. rsassa-pss-sha256
	Scheme() string
	// VerifySignature check the signature is right
	VerifySignature(payload []byte, sig string) error
}

// PrivKey is the private key that provide signature method
type PrivKey interface {
	Serializable
	// Type returns the type of the key, e.g. RSA
	Type() string
	// Scheme returns the scheme of  signature algorithm, e.g. rsassa-pss-sha256
	Scheme() string
	// Signature sign a signature with the key for payload
	Signature(payload []byte) (string, error)
	// Public returns public key of the PrivKey
	Public() PubKey
}

// NewKeyPair return a pair of key
func NewKeyPair(keyType, keyScheme string) (PubKey, PrivKey, error) {
	// We only support RSA now
	if keyType != KeyTypeRSA {
		return nil, nil, ErrorUnsupportedKeyType
	}

	// We only support rsassa-pss-sha256 now
	if keyScheme != KeySchemeRSASSAPSSSHA256 {
		return nil, nil, ErrorUnsupportedKeySchema
	}

	return RSAPair()
}

// NewPrivKey return PrivKey
func NewPrivKey(keyType, keyScheme string, key []byte) (PrivKey, error) {
	// We only support RSA now
	if keyType != KeyTypeRSA {
		return nil, ErrorUnsupportedKeyType
	}

	// We only support rsassa-pss-sha256 now
	if keyScheme != KeySchemeRSASSAPSSSHA256 {
		return nil, ErrorUnsupportedKeySchema
	}

	priv := &RSAPrivKey{}
	return priv, priv.Deserialize(key)
}

// NewPubKey return PrivKey
func NewPubKey(keyType, keyScheme string, key []byte) (PubKey, error) {
	// We only support RSA now
	if keyType != KeyTypeRSA {
		return nil, ErrorUnsupportedKeyType
	}

	// We only support rsassa-pss-sha256 now
	if keyScheme != KeySchemeRSASSAPSSSHA256 {
		return nil, ErrorUnsupportedKeySchema
	}

	pub := &RSAPubKey{}
	return pub, pub.Deserialize(key)
}
