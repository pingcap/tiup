package main

import (
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/utils"
)

// prepareComponentBinary ensures the resolved component version is installed and
// returns its executable path.
//
// It intentionally does not print "not installed; downloading..." messages.
// Playground already owns the download UX via the unified progress UI.
func prepareComponentBinary(component string, v utils.Version, forcePull bool) (string, error) {
	env := environment.GlobalEnv()
	if env == nil {
		return "", errors.New("global environment not initialized")
	}
	if component == "" {
		return "", errors.New("component is empty")
	}
	if v.IsEmpty() {
		return "", errors.Errorf("component `%s` version is empty", component)
	}

	needDownload := forcePull
	if !forcePull {
		_, err := env.SelectInstalledVersion(component, v)
		if err != nil {
			if errors.Cause(err) != environment.ErrInstallFirst {
				return "", err
			}
			needDownload = true
		}
	}

	if needDownload {
		spec := repository.ComponentSpec{
			ID:      component,
			Version: v.String(),
			Force:   forcePull,
		}
		if err := env.V1Repository().UpdateComponents([]repository.ComponentSpec{spec}); err != nil {
			return "", err
		}
	}

	return env.BinaryPath(component, v)
}
