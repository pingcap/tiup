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
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
)

// RSAKeyLength define the length of RSA keys
const RSAKeyLength = 2048

// RSAPair generate a pair of rsa keys
func RSAPair() (*RSAPubKey, *RSAPrivKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, RSAKeyLength)
	if err != nil {
		return nil, nil, err
	}
	return &RSAPubKey{&key.PublicKey}, &RSAPrivKey{key}, nil
}

// RSAPubKey represents the public key of RSA
type RSAPubKey struct {
	key *rsa.PublicKey
}

// Serialize generate the pem format for a key
func (k *RSAPubKey) Serialize() ([]byte, error) {
	asn1Bytes, err := asn1.Marshal(*k.key)
	if err != nil {
		return nil, err
	}
	pemKey := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: asn1Bytes,
	}
	return pem.EncodeToMemory(pemKey), nil
}

// Deserialize generate a public key from pem format
func (k *RSAPubKey) Deserialize(key []byte) error {
	block, _ := pem.Decode(key)
	if block == nil {
		return ErrorDeserializeKey
	}
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	k.key = pubInterface.(*rsa.PublicKey)
	return nil
}

// Verify check the signature is right
func (k *RSAPubKey) Verify(payload []byte, sig string) error {
	if k.key == nil {
		return ErrorKeyUninitialized
	}

	sha256 := crypto.SHA256.New()
	sha256.Write(payload)
	hashed := sha256.Sum(nil)

	b64decSig, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return err
	}

	return rsa.VerifyPSS(k.key, crypto.SHA256, hashed, b64decSig, nil)
}

// RSAPrivKey represents the private key of RSA
type RSAPrivKey struct {
	key *rsa.PrivateKey
}

// Serialize generate the pem format for a key
func (k *RSAPrivKey) Serialize() ([]byte, error) {
	pemKey := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(k.key),
	}

	return pem.EncodeToMemory(pemKey), nil
}

// Deserialize generate a private key from pem format
func (k *RSAPrivKey) Deserialize(key []byte) error {
	block, _ := pem.Decode(key)
	if block == nil {
		return ErrorDeserializeKey
	}
	PrivKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	k.key = PrivKey
	return nil
}

// Signature sign a signature with the key for payload
func (k *RSAPrivKey) Signature(payload []byte) (string, error) {
	if k.key == nil {
		return "", ErrorKeyUninitialized
	}

	sha256 := crypto.SHA256.New()
	sha256.Write(payload)
	hashed := sha256.Sum(nil)

	sig, err := rsa.SignPSS(rand.Reader, k.key, crypto.SHA256, hashed, nil)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// Public returns public key of the PrivKey
func (k *RSAPrivKey) Public() PubKey {
	return &RSAPubKey{
		key: &k.key.PublicKey,
	}
}
