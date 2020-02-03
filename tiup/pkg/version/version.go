package version

import (
	"fmt"
	"runtime"
)

var (
	// TiUPVerMajor is the major version of TiUP
	TiUPVerMajor = 0
	// TiUPVerMinor is the minor version of TiUP
	TiUPVerMinor = 0
	// TiUPVerPatch is the patch version of TiUP
	TiUPVerPatch = 0
	// TiUPVerName is alternative name of the version
	TiUPVerName = "Initial Demo"
	// BuildTime is the time when binary is built
	BuildTime = "Unknown"
	// GitHash is the current git commit hash
	GitHash = "Unknown"
	// GitBranch is the current git branch name
	GitBranch = "Unknown"
)

// TiUPVersion is the semver of TiUP
type TiUPVersion struct {
	major int
	minor int
	patch int
	name  string
}

// NewTiUPVersion creates a TiUPVersion object
func NewTiUPVersion() *TiUPVersion {
	return &TiUPVersion{
		major: TiUPVerMajor,
		minor: TiUPVerMinor,
		patch: TiUPVerPatch,
		name:  TiUPVerName,
	}
}

// Name returns the alternave name of TiUPVersion
func (v *TiUPVersion) Name() string {
	return v.name
}

// SemVer returns TiUPVersion in semver format
func (v *TiUPVersion) SemVer() string {
	return fmt.Sprintf("v%d.%d.%d", v.major, v.minor, v.patch)
}

// String converts TiUPVersion to a string
func (v *TiUPVersion) String() string {
	return fmt.Sprintf("v%d.%d.%d %s", v.major, v.minor, v.patch, v.name)
}

// TiUPBuild is the info of building environment
type TiUPBuild struct {
	BuildTime string `json:"buildTime"`
	GitHash   string `json:"gitHash"`
	GitBranch string `json:"gitBranch"`
	GoVersion string `json:"goVersion"`
}

// NewTiUPBuildInfo creates a TiUPBuild object
func NewTiUPBuildInfo() *TiUPBuild {
	return &TiUPBuild{
		BuildTime: BuildTime,
		GitHash:   GitHash,
		GitBranch: GitBranch,
		GoVersion: runtime.Version(),
	}
}
