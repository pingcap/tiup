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
)

// KeyStore is the collection of all public keys available to TiUp.
type KeyStore interface {
	// Add put new key into KeyStore
	Put(string, PubKey) KeyStore

	// Get return a key by it's id
	Get(string) PubKey
}

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
	// Verify check the signature is right
	Verify(payload []byte, sig string) error
}

// PrivKey is the private key that provide signature method
type PrivKey interface {
	Serializable
	// Signature sign a signature with the key for payload
	Signature(payload []byte) (string, error)
	// Public returns public key of the PrivKey
	Public() PubKey
}

type keychain map[string]PubKey

// NewKeyStore return a KeyStore
func NewKeyStore() KeyStore {
	return &keychain{}
}

// Get return key by id
func (s *keychain) Get(id string) PubKey {
	return (*s)[id]
}

// Add put a new key into keychain
func (s *keychain) Put(id string, key PubKey) KeyStore {
	(*s)[id] = key
	return s
}
