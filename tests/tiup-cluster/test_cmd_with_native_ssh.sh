#!/bin/bash

set -eu

source script/cmd_subtest.sh
export GO_FAILPOINTS='github.com/pingcap/tiup/pkg/cluster/executor/assertNativeSSH=return(true)' 

echo "test cluster for verision v4.0.2 with CDC, via native ssh"
cmd_subtest v4.0.2 true true

unset GO_FAILPOINTS
