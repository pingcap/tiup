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

package manager

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"path/filepath"

	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/crypto"
	"github.com/pingcap/tiup/pkg/utils"
)

func genAndSaveClusterCA(name, tlsPath string) (*crypto.CertificateAuthority, error) {
	ca, err := crypto.NewCA(name)
	if err != nil {
		return nil, err
	}

	// save CA private key
	if err := utils.SaveFileWithBackup(filepath.Join(tlsPath, spec.TLSCAKey), ca.Key.Pem(), ""); err != nil {
		return nil, perrs.Annotatef(err, "cannot save CA private key for %s", name)
	}

	// save CA certificate
	if err := utils.SaveFileWithBackup(
		filepath.Join(tlsPath, spec.TLSCACert),
		pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: ca.Cert.Raw,
		}), ""); err != nil {
		return nil, perrs.Annotatef(err, "cannot save CA certificate for %s", name)
	}

	return ca, nil
}

func genAndSaveClientCert(ca *crypto.CertificateAuthority, name, tlsPath string) error {
	privKey, err := crypto.NewKeyPair(crypto.KeyTypeRSA, crypto.KeySchemeRSASSAPSSSHA256)
	if err != nil {
		return err
	}

	// save client private key
	if err := utils.SaveFileWithBackup(filepath.Join(tlsPath, spec.TLSClientKey), privKey.Pem(), ""); err != nil {
		return perrs.Annotatef(err, "cannot save client private key for %s", name)
	}

	csr, err := privKey.CSR(
		"tiup-cluster-client",
		fmt.Sprintf("%s-client", name),
		[]string{}, []string{},
	)
	if err != nil {
		return perrs.Annotatef(err, "cannot generate CSR of client certificate for %s", name)
	}
	cert, err := ca.Sign(csr)
	if err != nil {
		return perrs.Annotatef(err, "cannot sign client certificate for %s", name)
	}

	// save client certificate
	if err := utils.SaveFileWithBackup(
		filepath.Join(tlsPath, spec.TLSClientCert),
		pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert,
		}), ""); err != nil {
		return perrs.Annotatef(err, "cannot save client PEM certificate for %s", name)
	}

	// save pfx format certificate
	clientCert, err := x509.ParseCertificate(cert)
	if err != nil {
		return perrs.Annotatef(err, "cannot decode signed client certificate for %s", name)
	}
	pfxData, err := privKey.PKCS12(clientCert, ca)
	if err != nil {
		return perrs.Annotatef(err, "cannot encode client certificate to PKCS#12 format for %s", name)
	}
	if err := utils.SaveFileWithBackup(
		filepath.Join(tlsPath, spec.PFXClientCert),
		pfxData,
		""); err != nil {
		return perrs.Annotatef(err, "cannot save client PKCS#12 certificate for %s", name)
	}

	return nil
}
