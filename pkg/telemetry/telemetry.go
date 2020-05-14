package telemetry

import (
	"io/ioutil"
	"os"

	"github.com/google/uuid"
	"github.com/pingcap/errors"
	"gopkg.in/yaml.v2"
)

// Status of telemetry.
type Status string

// Status of telemetry
const (
	EnableStatus  Status = "enable"
	DisableStatus Status = "disable"
)

const defaultStatus = EnableStatus

// Meta data of telemetry.
type Meta struct {
	UUID   string `yaml:"uuid,omitempty"`
	Status Status `yaml:"status,omitempty"`
}

// NewUUID return a new uuid.
func NewUUID() string {
	return uuid.New().String()
}

// NewMeta create a new default Meta.
func NewMeta() *Meta {
	return &Meta{
		UUID:   NewUUID(),
		Status: EnableStatus,
	}
}

// LoadFrom load meta from the specify file,
// return a default Meta and save it if the file not exist.
func LoadFrom(fname string) (meta *Meta, err error) {
	var data []byte
	data, err = ioutil.ReadFile(fname)

	if err != nil {
		if os.IsNotExist(err) {
			meta = NewMeta()
			return meta, meta.SaveTo(fname)
		}
		return
	}

	meta = new(Meta)
	err = yaml.Unmarshal(data, meta)

	return
}

// SaveTo save to the specified file.
func (m *Meta) SaveTo(fname string) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return errors.AddStack(err)
	}

	return ioutil.WriteFile(fname, data, 0644)
}
