package utils

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

// FmtVer converts a version string to SemVer format, if the string is not a valid
// SemVer and fails to parse and convert it, an error is raised.
func FmtVer(ver string) (string, error) {
	var v string
	if !strings.HasPrefix(ver, "v") {
		v = fmt.Sprintf("v%s", ver)
	}
	if !semver.IsValid(v) {
		return v, fmt.Errorf("version %s is not a valid SemVer string", ver)
	}
	return v, nil
}
