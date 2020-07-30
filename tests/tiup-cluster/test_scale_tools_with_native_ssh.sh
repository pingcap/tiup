#!/bin/bash

set -eu

source script/scale_tools.sh
export GO_FAILPOINTS='github.com/pingcap/tiup/pkg/cluster/executor/assertNativeSSH=return(true)' 

echo "test scaling of tools components in cluster for verision v4.0.2, via native ssh"
scale_tools v4.0.2 true

unset GO_FAILPOINTS
