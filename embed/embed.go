package embed

import (
	goembed "embed"
)

//go:embed templates
var embededFiles goembed.FS

// ReadFile read the file embed.
func ReadFile(path string) ([]byte, error) {
	return embededFiles.ReadFile(path)
}
