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

//go:debug rsa1024min=0
package v1manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	publicTestKey = []byte(`-----BEGIN PUBLIC KEY-----
MFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBALqbHeRLCyOdykC5SDLqI49ArYGYG1mq
aH9/GnWjGavZM02fos4lc2w6tCchcUBNtJvGqKwhC5JEnx3RYoSX2ucCAwEAAQ==
-----END PUBLIC KEY-----
`)

	privateTestKey = []byte(`
-----BEGIN RSA PRIVATE KEY-----

      MIIBPQIBAAJBALqbHeRLCyOdykC5SDLqI49ArYGYG1mqaH9/GnWjGavZM02fos4l
c2w6tCchcUBNtJvGqKwhC5JEnx3RYoSX2ucCAwEAAQJBAKn6O+tFFDt4MtBsNcDz
GDsYDjQbCubNW+yvKbn4PJ0UZoEebwmvH1ouKaUuacJcsiQkKzTHleu4krYGUGO1
mEECIQD0dUhj71vb1rN1pmTOhQOGB9GN1mygcxaIFOWW8znLRwIhAMNqlfLijUs6
rY+h1pJa/3Fh1HTSOCCCCWA0NRFnMANhAiEAwddKGqxPO6goz26s2rHQlHQYr47K
vgPkZu2jDCo7trsCIQC/PSfRsnSkEqCX18GtKPCjfSH10WSsK5YRWAY3KcyLAQIh
AL70wdUu5jMm2ex5cZGkZLRB50yE6rBiHCd5W1WdTFoe

-----END RSA PRIVATE KEY-----
`)
)

var cryptoCases = [][]byte{
	[]byte(`TiDB is an awesome database`),
	[]byte(`I like coding...`),
	[]byte(`I hate talking...`),
	[]byte(`Junk food is good`),
}

func TestKeyInfoIdentity(t *testing.T) {
	priv := NewKeyInfo(privateTestKey)
	require.True(t, priv.IsPrivate())

	pub1, err := priv.Public()
	require.Nil(t, err)
	pub2, err := priv.Public()
	require.Nil(t, err)
	pub3, err := pub2.Public()
	require.Nil(t, err)

	require.Equal(t, pub1.Value["public"], pub2.Value["public"])
	require.Equal(t, pub1.Value["public"], pub3.Value["public"])
	require.Equal(t, pub1.Value["public"], string(publicTestKey))

	id1, err := pub1.ID()
	require.Nil(t, err)
	id2, err := pub2.ID()
	require.Nil(t, err)
	id3, err := pub3.ID()
	require.Nil(t, err)

	require.Equal(t, id1, id2)
	require.Equal(t, id1, id3)
}

func TestKeyInfoID(t *testing.T) {
	priv := NewKeyInfo(privateTestKey)
	require.True(t, priv.IsPrivate())

	pub, err := priv.Public()
	require.Nil(t, err)
	require.True(t, !pub.IsPrivate())

	pubid, err := pub.ID()
	require.Nil(t, err)
	privid, err := pub.ID()
	require.Nil(t, err)
	require.NotEmpty(t, pubid)
	require.Equal(t, pubid, privid)
}

func TestKeyInfoSigAndVerify(t *testing.T) {
	pri := NewKeyInfo(privateTestKey)
	require.True(t, pri.IsPrivate())

	pub, err := pri.Public()
	require.Nil(t, err)

	for _, cas := range cryptoCases {
		sig, err := pri.Signature(cas)
		require.Nil(t, err)
		require.Nil(t, pub.Verify(cas, sig))
	}
}
