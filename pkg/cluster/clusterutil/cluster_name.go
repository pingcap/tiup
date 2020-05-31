// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package clusterutil

import (
	"regexp"

	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiup/pkg/cliutil"
	"github.com/pingcap-incubator/tiup/pkg/errutil"
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
