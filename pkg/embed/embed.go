package embed

import (
	"io/ioutil"

	"github.com/markbates/pkger"
	"github.com/pingcap/errors"
)

func init() {
	// Embeds all files under the specified path
	// Run `make pkger` to generate pkger.go if files changed.
	_ = pkger.Dir("/templates/config")
	_ = pkger.Dir("/templates/scripts")
	_ = pkger.Dir("/templates/systemd")
}

// ReadFile read the file embed.
func ReadFile(path string) ([]byte, error) {
	f, err := pkger.Open(path)
	if err != nil {
		return nil, errors.Annotatef(err, "read embed file: %s", path)
	}

	return ioutil.ReadAll(f)
}
