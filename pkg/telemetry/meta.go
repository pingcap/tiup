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

package telemetry

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/utils"
	"gopkg.in/yaml.v2"
)

const telemetryFname = "meta.yaml"

// Status of telemetry.
type Status string

// Status of telemetry
const (
	EnableStatus  Status = "enable"
	DisableStatus Status = "disable"
)

// Meta data of telemetry.
type Meta struct {
	UUID   string `yaml:"uuid,omitempty"`
	Secret string `yaml:"secret,omitempty"`
	Status Status `yaml:"status,omitempty"`
}

// NewUUID return a new uuid.
func NewUUID() string {
	return uuid.New().String()
}

// NewSecret generates a new random string as encryption secret
func NewSecret() string {
	rbytes := make([]byte, 16)
	_, err := rand.Read(rbytes)
	if err != nil {
		return NewUUID()
	}
	return fmt.Sprintf("%x", rbytes)
}

// NewMeta create a new default Meta.
func NewMeta() *Meta {
	return &Meta{
		UUID:   NewUUID(),
		Secret: NewSecret(),
		Status: EnableStatus,
	}
}

// LoadFrom load meta from the specify file,
// return a default Meta and save it if the file not exist.
func LoadFrom(fname string) (meta *Meta, err error) {
	var data []byte
	data, err = os.ReadFile(fname)

	if err != nil {
		if os.IsNotExist(err) {
			return &Meta{}, nil
		}
		return
	}

	meta = new(Meta)
	err = yaml.Unmarshal(data, meta)

	// populate UUID and secret if not set
	var updated bool
	if meta.UUID == "" && meta.Status == EnableStatus {
		meta.UUID = NewUUID()
		updated = true
	}
	if meta.Secret == "" && meta.Status == EnableStatus {
		meta.Secret = NewSecret()
		updated = true
	}
	if updated {
		err = meta.SaveTo(fname)
	}

	return
}

// SaveTo save to the specified file.
func (m *Meta) SaveTo(fname string) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return errors.AddStack(err)
	}

	return utils.WriteFile(fname, data, 0644)
}

// GetMeta read the telemeta from disk
func GetMeta(env *environment.Environment) (meta *Meta, fname string, err error) {
	dir := env.Profile().Path(localdata.TelemetryDir)
	err = utils.MkdirAll(dir, 0755)
	if err != nil {
		return
	}

	fname = filepath.Join(dir, telemetryFname)
	meta, err = LoadFrom(fname)
	return
}
