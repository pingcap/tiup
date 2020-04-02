package utils

import (
	"regexp"

	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiops/pkg/cliutil"
	"github.com/pingcap-incubator/tiops/pkg/errutil"
)

var (
	// ErrInvalidClusterName is an error for invalid cluster name. You should use `ValidateClusterNameOrError()`
	// to generate this error.
	ErrInvalidClusterName = errorx.CommonErrors.NewType("invalid_cluster_name", errutil.ErrTraitPreCheck)
)

var (
	clusterNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)
)

// ValidateClusterNameOrError validates a cluster name and returns error if the name is invalid.
func ValidateClusterNameOrError(n string) error {
	if len(n) == 0 {
		return ErrInvalidClusterName.
			New("Cluster name must not be empty")
	}
	if !clusterNameRegexp.MatchString(n) {
		return ErrInvalidClusterName.
			New("Cluster name '%s' is invalid", n).
			WithProperty(cliutil.SuggestionFromString("The cluster name should only contains alphabets, numbers, hyphen (-) and underscore (_)."))
	}
	return nil
}
