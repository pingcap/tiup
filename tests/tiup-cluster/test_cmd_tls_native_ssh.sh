#!/bin/bash

set -eu

source script/cmd_subtest.sh
export GO_FAILPOINTS='github.com/pingcap/tiup/pkg/cluster/executor/assertNativeSSH=return(true)'

echo "test cluster for version v4.0.12 w/ TLS, via native ssh"
cmd_subtest 6.0.0 true true

unset GO_FAILPOINTS
