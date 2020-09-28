#!/usr/bin/env bash

TEST_DIR=$(cd "$(dirname "$0")"; pwd)
TMP_DIR=$TEST_DIR/_tmp

rm -rf $TMP_DIR/data

# Profile home directory
mkdir -p $TMP_DIR/home/bin/
export TIUP_HOME=$TMP_DIR/home
curl https://tiup-mirrors.pingcap.com/root.json -o $TMP_DIR/home/bin/root.json

# Prepare data directory
mkdir -p $TMP_DIR/data
export TIUP_INSTANCE_DATA_DIR=$TMP_DIR/data

mkdir -p $TEST_DIR/cover

function tiup() {
    # echo "in function"
    if [ -f "$TEST_DIR/bin/tiup.test" ]; then
        $TEST_DIR/bin/tiup.test -test.coverprofile=$TEST_DIR/cover/cov.itest-$(date +'%s')-$RANDOM.out __DEVEL--i-heard-you-like-tests "$@"
    else
        $TEST_DIR/../../bin/tiup "$@"
    fi
}

tiup list
tiup
tiup help
tiup install tidb:v3.0.13
tiup update tidb
tiup update --self
tiup status
tiup clean --all
tiup help tidb
tiup env
TIUP_SSHPASS_PROMPT="password" tiup env TIUP_SSHPASS_PROMPT | grep password
tiup uninstall
tiup uninstall tidb:v3.0.13
tiup uninstall tidb --all
tiup uninstall --all
tiup uninstall --self
