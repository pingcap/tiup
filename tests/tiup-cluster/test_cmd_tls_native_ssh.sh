#!/bin/bash

set -eu

source script/cmd_subtest.sh
export GO_FAILPOINTS='github.com/pingcap/tiup/pkg/cluster/executor/assertNativeSSH=return(true)'

echo "test cluster for version v6.0.0 w/ TLS, via native ssh"
cmd_subtest --version 6.0.0 --tls --native-ssh

unset GO_FAILPOINTS
