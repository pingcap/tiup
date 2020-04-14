#!/bin/bash

set -eu

# Change directory to the source directory of this script. Taken from:
# https://stackoverflow.com/a/246128/3858681
pushd "$( cd "$( dirname "${BASH_SOURCE[0]}"  )" >/dev/null 2>&1 && pwd  )"

PATH=$PATH:/tiup-cluster/bin
export TIUP_CLUSTER_PROGRESS_REFRESH_RATE=10s
export TIUP_CLUSTER_EXECUTE_DEFAULT_TIMEOUT=300s

. ./script/util.sh

# TODO remove this once embed the files in binary
# the work dir of tiup-cluster need this
ln -s ../templates templates || true

. ./test_cmd.sh v4.0.0-rc


echo "\033[0;36m<<< Run all test success >>>\033[0m"
