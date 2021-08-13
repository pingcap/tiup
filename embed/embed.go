package embed

import (
	goembed "embed"
)

//go:embed templates
var embededFiles goembed.FS

// ReadTemplate read the template file embed.
func ReadTemplate(path string) ([]byte, error) {
	return embededFiles.ReadFile(path)
}

//go:embed examples
var embedExamples goembed.FS

// ReadExample read an example file
func ReadExample(path string) ([]byte, error) {
	return embedExamples.ReadFile(path)
}
