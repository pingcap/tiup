#!/bin/bash

set -eu

# Change directory to the source directory of this script. Taken from:
# https://stackoverflow.com/a/246128/3858681
pushd "$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

PATH=$PATH:/tiup-cluster/bin
export TIUP_CLUSTER_PROGRESS_REFRESH_RATE=10s
export TIUP_CLUSTER_EXECUTE_DEFAULT_TIMEOUT=300s
export DEBUG_CHECKPOINT=1

export version=${version-v4.0.12}
export old_version=${old_version-v3.0.20}

# Prepare local config
echo "preparing local config"
ls -lh ./local
rm -rf /tmp/local
cp -r ./local /tmp/local
ls -lh /tmp/local

function tiup-cluster() {
    mkdir -p ~/.tiup/bin && cp -f ./root.json ~/.tiup/bin/
    # echo "in function"
    if [ -f "./bin/tiup-cluster.test" ]; then
        ./bin/tiup-cluster.test -test.coverprofile=./cover/cov.itest-$(date +'%s')-$RANDOM.out __DEVEL--i-heard-you-like-tests "$@"
    else
        ../../bin/tiup-cluster "$@"
    fi
}

function tiup() {
    mkdir -p ~/.tiup/bin && cp -f ./root.json ~/.tiup/bin/
    if [ -f "../tiup/bin/tiup.test" ]; then
        ../tiup/bin/tiup.test -test.coverprofile=./cover/cov.itest-$(date +'%s')-$RANDOM.out __DEVEL--i-heard-you-like-tests "$@"
    else
        ../../bin/tiup "$@"
    fi
}

. ./script/util.sh

# use run.sh test_cmd test_upgrade to run specify cases
do_cases=$*

if [ "$do_cases" == "" ]; then
    for script in ./test_*.sh; do
        echo "run test: $script"
        . $script
    done
else
    for script in "${do_cases[@]}"; do
        echo "run test: $script.sh"
        . ./$script.sh
    done
fi

echo -e "\033[0;36m<<< Run all test success >>>\033[0m"
