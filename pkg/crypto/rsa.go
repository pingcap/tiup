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
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"net"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/crypto/rand"
	"software.sslmate.com/src/go-pkcs12"
)

// RSAKeyLength define the length of RSA keys
const RSAKeyLength = 2048

// RSAPair generate a pair of rsa keys
func RSAPair() (*RSAPrivKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, RSAKeyLength)
	if err != nil {
		return nil, err
	}
	return &RSAPrivKey{key}, nil
}

// RSAPubKey represents the public key of RSA
type RSAPubKey struct {
	key *rsa.PublicKey
}

// Type returns the type of the key, e.g. RSA
func (k *RSAPubKey) Type() string {
	return KeyTypeRSA
}

// Scheme returns the scheme of  signature algorithm, e.g. rsassa-pss-sha256
func (k *RSAPubKey) Scheme() string {
	return KeySchemeRSASSAPSSSHA256
}

// Key returns the raw public key
func (k *RSAPubKey) Key() crypto.PublicKey {
	return k.key
}

// Serialize generate the pem format for a key
func (k *RSAPubKey) Serialize() ([]byte, error) {
	asn1Bytes, err := x509.MarshalPKIXPublicKey(k.key)
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

// VerifySignature check the signature is right
func (k *RSAPubKey) VerifySignature(payload []byte, sig string) error {
	if k.key == nil {
		return ErrorKeyUninitialized
	}

	sha256 := crypto.SHA256.New()
	_, err := sha256.Write(payload)
	if err != nil {
		return errors.AddStack(err)
	}

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

// Type returns the type of the key, e.g. RSA
func (k *RSAPrivKey) Type() string {
	return KeyTypeRSA
}

// Scheme returns the scheme of  signature algorithm, e.g. rsassa-pss-sha256
func (k *RSAPrivKey) Scheme() string {
	return KeySchemeRSASSAPSSSHA256
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
	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	k.key = privKey
	return nil
}

// Signature sign a signature with the key for payload
func (k *RSAPrivKey) Signature(payload []byte) (string, error) {
	if k.key == nil {
		return "", ErrorKeyUninitialized
	}

	sha256 := crypto.SHA256.New()
	_, err := sha256.Write(payload)
	if err != nil {
		return "", errors.AddStack(err)
	}

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

// Signer returns the signer of the private key
func (k *RSAPrivKey) Signer() crypto.Signer {
	return k.key
}

// Pem returns the raw private key im PEM format
func (k *RSAPrivKey) Pem() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(k.key),
	})
}

// CSR generates a new CSR from given private key
func (k *RSAPrivKey) CSR(role, commonName string, hostList, ipList []string) ([]byte, error) {
	var ipAddrList []net.IP
	for _, ip := range ipList {
		ipAddr := net.ParseIP(ip)
		ipAddrList = append(ipAddrList, ipAddr)
	}

	// set CSR attributes
	csrTemplate := &x509.CertificateRequest{
		Subject: pkix.Name{
			Organization:       []string{pkixOrganization},
			OrganizationalUnit: []string{pkixOrganizationalUnit, role},
			CommonName:         commonName,
		},
		DNSNames:    hostList,
		IPAddresses: ipAddrList,
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, csrTemplate, k.key)
	if err != nil {
		return nil, err
	}

	return csr, nil
}

// PKCS12 encodes the private and certificate to a PKCS#12 pfxData
func (k *RSAPrivKey) PKCS12(cert *x509.Certificate, ca *CertificateAuthority) ([]byte, error) {
	return pkcs12.Encode(
		rand.Reader,
		k.key,
		cert,
		[]*x509.Certificate{ca.Cert},
		PKCS12Password,
	)
}
