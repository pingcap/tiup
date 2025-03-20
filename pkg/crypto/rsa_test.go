//go:debug rsa1024min=0
package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var cases = [][]byte{
	[]byte(`TiDB is an awesome database`),
	[]byte(`I like coding...`),
	[]byte(`I hate talking...`),
	[]byte(`Junk food is good`),
}

var (
	publicTestKey = []byte(`
-----BEGIN PUBLIC KEY-----
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

func TestSignAndVerify(t *testing.T) {
	priv, err := RSAPair()
	assert.Nil(t, err)

	for _, cas := range cases {
		sig, err := priv.Signature(cas)
		assert.Nil(t, err)
		assert.Nil(t, priv.Public().VerifySignature(cas, sig))
	}
}

func TestSeriAndDeseri(t *testing.T) {
	pub := RSAPubKey{}
	pri := RSAPrivKey{}

	_, err := pri.Signature([]byte("foo"))
	assert.EqualError(t, err, ErrorKeyUninitialized.Error())

	assert.EqualError(t, pub.VerifySignature([]byte(`foo`), "sig"), ErrorKeyUninitialized.Error())

	assert.Nil(t, pub.Deserialize(publicTestKey))
	assert.Nil(t, pri.Deserialize(privateTestKey))

	for _, cas := range cases {
		sig, err := pri.Signature(cas)
		assert.Nil(t, err)
		assert.Nil(t, pub.VerifySignature(cas, sig))
	}
}
