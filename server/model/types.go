package model

import (
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
)

type signature struct {
	KeyID string `json:"keyid"`
	Sig   string `json:"sig"`
}

// ComponentManifest represents xxx.json
type ComponentManifest struct {
	// Signatures value
	Signatures []signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Component `json:"signed"`
}

// RootManifest represents root.json
type RootManifest struct {
	// Signatures value
	Signatures []signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Root `json:"signed"`
}

// IndexManifest represents index.json
type IndexManifest struct {
	// Signatures value
	Signatures []signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Index `json:"signed"`
}

// SnapshotManifest represents snapshot.json
type SnapshotManifest struct {
	// Signatures value
	Signatures []signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Snapshot `json:"signed"`
}

// TimestampManifest represents timestamp.json
type TimestampManifest struct {
	// Signatures value
	Signatures []signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed v1manifest.Timestamp `json:"signed"`
}
