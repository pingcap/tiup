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

package v1manifest

import (
	"strings"
	"testing"

	"github.com/pingcap-incubator/tiup/pkg/repository/crypto"
	"github.com/stretchr/testify/assert"
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

func TestReadTimestamp(t *testing.T) {
	pubKey := &crypto.RSAPubKey{}
	assert.Nil(t, pubKey.Deserialize(publicTestKey))

	_, err := readTimestampManifest(strings.NewReader(`{
		"signatures":[
			{
				"keyid": "test-key-id",
				"sig": "rZJGhKtBxz1Hp3Fnz8ZO7DSqOgpVQSn3OhwyAFrWlbKTvCThBaE5Y2qYxkJcuckIaoDRp4smwEtwPX/GMpFmdg=="
			}
		],
		"signed": {
			"_type": "timestamp",
			"spec_version": "0.1.0",
			"expires": "2220-05-11T04:51:08Z",
			"version": 43,
			"meta": {
				"/snapshot.json": {
					"hashes": {
						"sha256": "TODO"
					},
					"length": 515
				}
			}
		}
	}`), crypto.NewKeyStore().Put("test-key-id", pubKey))
	assert.Nil(t, err)
}

func TestEmptyManifest(t *testing.T) {
	var ts Timestamp
	_, err := ReadManifest(strings.NewReader(""), &ts, nil)
	assert.NotNil(t, err)
}

func TestWriteManifest(t *testing.T) {
	ts := Timestamp{Meta: map[string]FileHash{ManifestURLSnapshot: {
		Hashes: map[string]string{"TODO": "TODO"},
		Length: 0,
	}},
		SignedBase: SignedBase{
			Ty:          ManifestTypeTimestamp,
			SpecVersion: "0.1.0",
			Expires:     "2220-05-11T04:51:08Z",
			Version:     10,
		}}
	var out strings.Builder
	_, priv, err := crypto.RSAPair()
	assert.Nil(t, err)
	bytes, err := priv.Serialize()
	assert.Nil(t, err)

	err = SignAndWrite(&out, &ts, NewKeyInfo(bytes))
	assert.Nil(t, err)
	ts2, err := readTimestampManifest(strings.NewReader(out.String()), nil)
	assert.Nil(t, err)
	assert.Equal(t, *ts2, ts)
}

// TODO test that invalid manifests trigger errors
// TODO test SignAndWrite
