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
MIGeMA0GCSqGSIb3DQEBAQUAA4GMADCBiAKBgF6a4nJojBUcNmxWu7nXFBjlUew3
Se2N3Wqj3BVLYwPWOsrGynPHXh1MF3naIenVty+mQlfqfC/RAFtR31ImHQDBOG2Y
mQ/gzxHFWUarmR2nNF8DCbjF9D2JOCStisx79sB0LzF0/7nLEeivRv9lgIQZgOG5
Z0QlIzzy3Ymxu5s1AgMBAAE=
-----END PUBLIC KEY-----
`)

	privateTestKey = []byte(`
-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgF6a4nJojBUcNmxWu7nXFBjlUew3Se2N3Wqj3BVLYwPWOsrGynPH
Xh1MF3naIenVty+mQlfqfC/RAFtR31ImHQDBOG2YmQ/gzxHFWUarmR2nNF8DCbjF
9D2JOCStisx79sB0LzF0/7nLEeivRv9lgIQZgOG5Z0QlIzzy3Ymxu5s1AgMBAAEC
gYBBuoCMFoEFBbX2LYh+BKWM6n6xjHRLnN3yAmidTuQ7PTNZwSXVrPWBi2VgHqKj
UP3WGEBNzrd7jU0fJVHwRFSvXvTNho5JyWIACpxu7+KQ6X83hxLR9hM4bMIP19Qg
qNgIdU2OvAPKmtv4CM8VNTlDeB7HpdZNwJh6BFp6ykidHQJBALmPZpa2oO3PZ31g
9V3Q6KY5kvuSbuGG879y9S/+WnP2tY7VLjPHkyhAcLxEdtsR/SJ+xwYYoeQLOcqE
twm91MMCQQCChIfEZP4PnJvyS3Zv85hvC/Bxcb0+2tpMVBbzILsSFppnMs13+kBn
qyAF/ugpvKJFLHEFjOW9P+p7eEXv5fCnAkEAoXTlDr5ZyJJuueljlf3wcLIn8j23
vQRvkmW0cc4fZkeEMoPLb8J3iM6JSUdJI9TDLQCiq+tC8enSnyRbH17NgQJAKFkY
L5qY//KGMy0o/AruQMYMGsXynw/BFH+aaKbhrgHW0bhe1IxEhMfeKnxXATATahcH
CZQ5IXw03N6doEARWQJBAKy+MMXxB7D4CM6qYBCPvzQD/MLFxQCJCrf7r2a3OSHv
xszs/Mo/8gc28hoBwrkWBIUjY5leRR2TIZnGzZ1tZZk=
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
