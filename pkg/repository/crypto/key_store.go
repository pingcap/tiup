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

// KeyStore is the collection of all public keys available to TiUp.
type KeyStore interface {
	// KeyIDs returns all key ids
	KeyIDs() []string

	// Add put new key into KeyStore
	Put(string, PubKey) KeyStore

	// Get return a key by it's id
	Get(string) PubKey

	// Visit applies the supplied function to all keys
	Visit(func(id string, key PubKey))
}

type keychain map[string]PubKey

// NewKeyStore return a KeyStore
func NewKeyStore() KeyStore {
	return &keychain{}
}

var _ KeyStore = &keychain{}

// Visit implements KeyStore interface.
func (s *keychain) Visit(fn func(id string, key PubKey)) {
	for id, key := range *s {
		fn(id, key)
	}
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

// Keys returns all key ids
func (s *keychain) KeyIDs() []string {
	ids := []string{}
	for id := range *s {
		ids = append(ids, id)
	}
	return ids
}
