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
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"
)

var serialNumberLimit = new(big.Int).Lsh(big.NewInt(1), 128)

// CertificateAuthority is the CA of a cluster
type CertificateAuthority struct {
	ClusterName string
	Cert        *x509.Certificate
	Key         crypto.Signer
}

// NewCA generates a new CA certificate
func NewCA(clsName string) (*CertificateAuthority, error) {
	currTime := time.Now().UTC()

	// generate a random serial number for the new ca
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	caTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization:       []string{pkixOrganization},
			OrganizationalUnit: []string{pkixOrganizationalUnit, clsName},
		},
		NotBefore: currTime,
		NotAfter:  currTime.Add(time.Hour * 24 * 365 * 50), // TODO: support ca cert rotate
		IsCA:      true,
		KeyUsage:  x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
	}

	pubKey, privKey, err := RSAPair()
	if err != nil {
		return nil, err
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, pubKey.key, privKey.key)
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
		Key:         privKey.key,
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
		return nil, fmt.Errorf("the signer has expired: NotAfter=%v", ca.Cert.NotAfter)
	}

	// generate a random serial number for the new cert
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
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

	return x509.CreateCertificate(rand.Reader, template, ca.Cert, csr.PublicKey, ca.Key)
}
