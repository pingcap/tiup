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
	"testing"

	"github.com/alecthomas/assert"
)

var cryptoCases = [][]byte{
	[]byte(`TiDB is an awesome database`),
	[]byte(`I like coding...`),
	[]byte(`I hate talking...`),
	[]byte(`Junk food is good`),
}

func TestKeyInfoIdentity(t *testing.T) {
	priv := NewKeyInfo(privateTestKey)
	assert.True(t, priv.IsPrivate())

	pub1, err := priv.Public()
	assert.Nil(t, err)
	pub2, err := priv.Public()
	assert.Nil(t, err)
	pub3, err := pub2.Public()
	assert.Nil(t, err)

	assert.Equal(t, pub1.Value["public"], pub2.Value["public"])
	assert.Equal(t, pub1.Value["public"], pub3.Value["public"])
	assert.Equal(t, pub1.Value["public"], string(publicTestKey))

	id1, err := pub1.ID()
	assert.Nil(t, err)
	id2, err := pub2.ID()
	assert.Nil(t, err)
	id3, err := pub3.ID()
	assert.Nil(t, err)

	assert.Equal(t, id1, id2)
	assert.Equal(t, id1, id3)
}

func TestKeyInfoID(t *testing.T) {
	priv := NewKeyInfo(privateTestKey)
	assert.True(t, priv.IsPrivate())

	pub, err := priv.Public()
	assert.Nil(t, err)
	assert.True(t, !pub.IsPrivate())

	pubid, err := pub.ID()
	assert.Nil(t, err)
	privid, err := pub.ID()
	assert.Nil(t, err)
	assert.NotEmpty(t, pubid)
	assert.Equal(t, pubid, privid)
}

func TestKeyInfoSigAndVerify(t *testing.T) {
	pri := NewKeyInfo(privateTestKey)
	assert.True(t, pri.IsPrivate())

	pub, err := pri.Public()
	assert.Nil(t, err)

	for _, cas := range cryptoCases {
		sig, err := pri.Signature(cas)
		assert.Nil(t, err)
		assert.Nil(t, pub.Verify(cas, sig))
	}
}
