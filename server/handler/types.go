package handler

import (
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
)

type simpleResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type signature struct {
	KeyID string `json:"keyid"`
	Sig   string `json:"sig"`
}

type componentManifest struct {
	// Signatures value
	Signatures []signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Component `json:"signed"`
}

type indexManifest struct {
	// Signatures value
	Signatures []signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Index `json:"signed"`
}

type snapshotManifest struct {
	// Signatures value
	Signatures []signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Snapshot `json:"signed"`
}

type timestampManifest struct {
	// Signatures value
	Signatures []signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Timestamp `json:"signed"`
}
