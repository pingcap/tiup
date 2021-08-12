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
	cr "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/crypto/rand"
)

var serialNumberLimit = new(big.Int).Lsh(big.NewInt(1), 128)

// CertificateAuthority holds the CA of a cluster
type CertificateAuthority struct {
	ClusterName string
	Cert        *x509.Certificate
	Key         PrivKey
}

// NewCA generates a new CertificateAuthority object
func NewCA(clsName string) (*CertificateAuthority, error) {
	currTime := time.Now().UTC()

	// generate a random serial number for the new ca
	serialNumber, err := cr.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	caTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		// NOTE: not adding cluster name to the cert subject to avoid potential issues
		// when we implement cluster renaming feature. We may consider add this back
		// if we find proper way renaming a TLS enabled cluster.
		// Adding the cluster name in cert subject may be helpful to diagnose problem
		// when a process is trying to connecting a component from another cluster.
		Subject: pkix.Name{
			Organization:       []string{pkixOrganization},
			OrganizationalUnit: []string{pkixOrganizationalUnit /*, clsName */},
		},
		NotBefore: currTime,
		NotAfter:  currTime.Add(time.Hour * 24 * 365 * 50), // TODO: support ca cert rotate
		IsCA:      true,                                    // must be true
		KeyUsage:  x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
	}

	priv, err := NewKeyPair(KeyTypeRSA, KeySchemeRSASSAPSSSHA256)
	if err != nil {
		return nil, err
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, priv.Public().Key(), priv.Signer())
	if err != nil {
		return nil, err
	}
	caCert, err := x509.ParseCertificate(caBytes)
	if err != nil {
		return nil, err
	}
	return &CertificateAuthority{
		ClusterName: clsName,
		Cert:        caCert,
		Key:         priv,
	}, nil
}

// Sign signs a CSR with the CA
func (ca *CertificateAuthority) Sign(csrBytes []byte) ([]byte, error) {
	csr, err := x509.ParseCertificateRequest(csrBytes)
	if err != nil {
		return nil, err
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, err
	}

	currTime := time.Now().UTC()
	if !currTime.Before(ca.Cert.NotAfter) {
		return nil, errors.Errorf("the signer has expired: NotAfter=%v", ca.Cert.NotAfter)
	}

	// generate a random serial number for the new cert
	serialNumber, err := cr.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		Signature:          csr.Signature,
		SignatureAlgorithm: csr.SignatureAlgorithm,
		PublicKey:          csr.PublicKey,
		PublicKeyAlgorithm: csr.PublicKeyAlgorithm,

		SerialNumber:   serialNumber,
		Issuer:         ca.Cert.Issuer,
		Subject:        csr.Subject,
		DNSNames:       csr.DNSNames,
		IPAddresses:    csr.IPAddresses,
		EmailAddresses: csr.EmailAddresses,
		URIs:           csr.URIs,
		NotBefore:      currTime,
		NotAfter:       currTime.Add(time.Hour * 24 * 365 * 10),
		KeyUsage:       x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		Extensions:      csr.Extensions,
		ExtraExtensions: csr.ExtraExtensions,
	}

	return x509.CreateCertificate(rand.Reader, template, ca.Cert, csr.PublicKey, ca.Key.Signer())
}

// ReadCA reads an existing CA certificate from disk
func ReadCA(clsName, certPath, keyPath string) (*CertificateAuthority, error) {
	// read private key
	rawKey, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, errors.Annotatef(err, "error reading CA private key for %s", clsName)
	}
	keyPem, _ := pem.Decode(rawKey)
	if keyPem == nil {
		return nil, errors.Errorf("error decoding CA private key for %s", clsName)
	}
	var privKey PrivKey
	switch keyPem.Type {
	case "RSA PRIVATE KEY":
		pk, err := x509.ParsePKCS1PrivateKey(keyPem.Bytes)
		if err != nil {
			return nil, errors.Annotatef(err, "error decoding CA private key for %s", clsName)
		}
		privKey = &RSAPrivKey{key: pk}
	default:
		return nil, errors.Errorf("the CA private key type \"%s\" is not supported", keyPem.Type)
	}

	// read certificate
	rawCert, err := os.ReadFile(certPath)
	if err != nil {
		return nil, errors.Annotatef(err, "error reading CA certificate for %s", clsName)
	}
	certPem, _ := pem.Decode(rawCert)
	if certPem == nil {
		return nil, errors.Errorf("error decoding CA certificate for %s", clsName)
	}
	if certPem.Type != "CERTIFICATE" {
		return nil, errors.Errorf("the CA certificate type \"%s\" is not valid", certPem.Type)
	}
	cert, err := x509.ParseCertificate(certPem.Bytes)
	if err != nil {
		return nil, errors.Annotatef(err, "error decoding CA certificate for %s", clsName)
	}

	return &CertificateAuthority{
		ClusterName: clsName,
		Cert:        cert,
		Key:         privKey,
	}, nil
}
