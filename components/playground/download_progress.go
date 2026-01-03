package main

import (
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/repository"
	progressv2 "github.com/pingcap/tiup/pkg/tuiv2/progress"
)

func newRepoDownloadProgress(g *progressv2.Group) repository.DownloadProgress {
	return &repoDownloadProgress{group: g}
}

// repoDownloadProgress adapts repository download callbacks into the unified
// progress UI used by playground.
//
// It intentionally lives in playground (not tuiv2) so tuiv2 stays free of any
// repository-specific conventions (like tarball naming rules).
type repoDownloadProgress struct {
	group *progressv2.Group

	mu   sync.Mutex
	task *progressv2.Task
}

func (p *repoDownloadProgress) Start(rawURL string, size int64) {
	if p == nil || p.group == nil {
		return
	}

	name, version := downloadDisplay(rawURL)

	t := p.group.Task(name)
	if version != "" {
		t.SetMeta(version)
	}
	if size > 0 {
		t.SetTotal(size)
	}
	t.SetKindDownload()

	p.mu.Lock()
	p.task = t
	p.mu.Unlock()
}

func (p *repoDownloadProgress) SetCurrent(size int64) {
	p.mu.Lock()
	t := p.task
	p.mu.Unlock()

	if t == nil {
		return
	}
	t.SetCurrent(size)
}

func (p *repoDownloadProgress) Finish() {
	p.mu.Lock()
	t := p.task
	p.task = nil
	p.mu.Unlock()

	if t == nil {
		return
	}
	t.Done()
}

func downloadDisplay(rawURL string) (name, version string) {
	base := downloadTitle(rawURL)
	component, v, ok := parseComponentVersionFromTarball(base)
	if !ok {
		// Fallback to the raw filename to avoid hiding information for unknown
		// patterns.
		return base, ""
	}
	return proc.ComponentDisplayName(proc.RepoComponentID(component)), v
}

func downloadTitle(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err == nil && u.Path != "" {
		if base := path.Base(u.Path); base != "" && base != "." && base != "/" {
			return base
		}
	}
	// Fallback: best-effort base name on the original string.
	if base := path.Base(rawURL); base != "" && base != "." && base != "/" {
		return base
	}
	return rawURL
}

func parseComponentVersionFromTarball(filename string) (component, version string, ok bool) {
	name := strings.TrimSuffix(filename, ".tar.gz")
	if name == filename {
		// Only attempt parsing on the known tiup tarball naming convention.
		return "", "", false
	}

	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return "", "", false
	}

	// Drop the trailing platform suffix ("<goos>-<goarch>") when present.
	if len(parts) >= 4 && isKnownGOOS(parts[len(parts)-2]) && isKnownGOARCH(parts[len(parts)-1]) {
		parts = parts[:len(parts)-2]
	}

	versionStart := -1
	for i := 1; i < len(parts); i++ {
		if looksLikeVersionPart(parts[i]) {
			versionStart = i
			break
		}
	}
	if versionStart <= 0 {
		return "", "", false
	}

	component = strings.Join(parts[:versionStart], "-")
	version = strings.Join(parts[versionStart:], "-")
	return component, version, true
}

func looksLikeVersionPart(part string) bool {
	if part == "" {
		return false
	}
	switch part {
	case "nightly", "latest":
		return true
	}
	// Common semver form is either "v1.2.3" or "1.2.3".
	if len(part) >= 2 && part[0] == 'v' && part[1] >= '0' && part[1] <= '9' {
		return true
	}
	if part[0] >= '0' && part[0] <= '9' {
		return true
	}
	return false
}

func isKnownGOOS(goos string) bool {
	switch goos {
	case "linux", "darwin", "windows":
		return true
	default:
		return false
	}
}

func isKnownGOARCH(goarch string) bool {
	switch goarch {
	case "amd64", "arm64", "arm", "386", "ppc64le", "s390x", "riscv64":
		return true
	default:
		return false
	}
}

var _ repository.DownloadProgress = (*repoDownloadProgress)(nil)
