package embed

import (
	"encoding/base64"
	"os"
)

// ReadFile read the file embed.
func ReadFile(path string) ([]byte, error) {
	content, found := autogenFiles[path]
	if !found {
		return nil, os.ErrNotExist
	}
	return base64.StdEncoding.DecodeString(content)
}
