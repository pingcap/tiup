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
	"crypto/x509"
	"fmt"
	"testing"

	"slices"

	"github.com/stretchr/testify/assert"
)

func TestNewCA(t *testing.T) {
	clsName := "testing-ca"
	ca, err := NewCA(clsName)
	assert.Nil(t, err)
	assert.NotEmpty(t, ca.Cert)
	assert.NotEmpty(t, ca.Key)

	// check if it's a CA cert
	assert.True(t, ca.Cert.IsCA)

	// check for key subject
	assert.NotEmpty(t, ca.Cert.Subject.Organization)
	assert.Equal(t, pkixOrganization, ca.Cert.Subject.Organization[0])
	assert.NotEmpty(t, ca.Cert.Subject.OrganizationalUnit)
	assert.Equal(t, pkixOrganizationalUnit, ca.Cert.Subject.OrganizationalUnit[0])
	// assert.Equal(t, clsName, ca.Cert.Subject.OrganizationalUnit[1])

	// check for key usage
	assert.Equal(t, x509.KeyUsage(33), ca.Cert.KeyUsage) // x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature

	// check for extended usage
	err = func(cert *x509.Certificate) error {
		for _, usage := range []x509.ExtKeyUsage{ // expected extended key usage list
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		} {
			if func(a x509.ExtKeyUsage, s []x509.ExtKeyUsage) bool {
				return slices.Contains(s, a)
			}(usage, cert.ExtKeyUsage) {
				continue
			}
			return fmt.Errorf("extended key usage %v not found in generated CA cert", usage)
		}
		return nil
	}(ca.Cert)
	assert.Nil(t, err)
}

func TestCASign(t *testing.T) {
	// generate ca
	ca, err := NewCA("testing-ca")
	assert.Nil(t, err)

	// generate cert
	privKey, err := NewKeyPair(KeyTypeRSA, KeySchemeRSASSAPSSSHA256)
	assert.Nil(t, err)

	csr, err := privKey.CSR("tidb", "testing-cn",
		[]string{
			"tidb-server",
			"tidb.server.local",
		}, []string{
			"10.0.0.1",
			"1.2.3.4",
			"fe80:2333::dead:beef",
			"2403:5180:5:c37d::",
		})
	assert.Nil(t, err)

	certBytes, err := ca.Sign(csr)
	assert.Nil(t, err)
	cert, err := x509.ParseCertificate(certBytes)
	assert.Nil(t, err)

	assert.False(t, cert.IsCA)
	assert.Equal(t, ca.Cert.Issuer, cert.Issuer)
	assert.Equal(t, []string{pkixOrganization}, cert.Subject.Organization)
	assert.Equal(t, []string{pkixOrganizationalUnit, "tidb"}, cert.Subject.OrganizationalUnit)
	assert.Equal(t, "testing-cn", cert.Subject.CommonName)
	assert.Equal(t, []string{"tidb-server", "tidb.server.local"}, cert.DNSNames)
	assert.Equal(t, "10.0.0.1", cert.IPAddresses[0].String())
	assert.Equal(t, "1.2.3.4", cert.IPAddresses[1].String())
	assert.Equal(t, "fe80:2333::dead:beef", cert.IPAddresses[2].String())
	assert.Equal(t, "2403:5180:5:c37d::", cert.IPAddresses[3].String())

	// check for key usage
	assert.Equal(t, x509.KeyUsage(5), cert.KeyUsage) // x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment

	// check for extended usage
	err = func(cert *x509.Certificate) error {
		for _, usage := range []x509.ExtKeyUsage{ // expected extended key usage list
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		} {
			if func(a x509.ExtKeyUsage, s []x509.ExtKeyUsage) bool {
				return slices.Contains(s, a)
			}(usage, cert.ExtKeyUsage) {
				continue
			}
			return fmt.Errorf("extended key usage %v not found in signed cert", usage)
		}
		return nil
	}(cert)
	assert.Nil(t, err)
}
