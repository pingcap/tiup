#!/usr/bin/env bash

set -eu

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
tiup update tidb --nightly
tiup --binary tidb:nightly
tiup status
tiup clean --all
tiup help tidb
tiup env
TIUP_SSHPASS_PROMPT="password" tiup env TIUP_SSHPASS_PROMPT | grep password

# test mirror
cat > /tmp/hello.sh << EOF
#! /bin/sh

echo "hello, TiDB"
EOF
chmod 755 /tmp/hello.sh
tar -C /tmp -czf /tmp/hello.tar.gz hello.sh

tiup mirror genkey

tiup mirror init /tmp/test-mirror-a
tiup mirror set /tmp/test-mirror-a
tiup mirror grant pingcap
echo "should fail"
! tiup mirror grant pingcap # this should failed
tiup mirror publish hello v0.0.1 /tmp/hello.tar.gz hello.sh
tiup hello:v0.0.1 | grep TiDB

tiup mirror init /tmp/test-mirror-b
tiup mirror set /tmp/test-mirror-b
tiup mirror grant pingcap
tiup mirror publish hello v0.0.2 /tmp/hello.tar.gz hello.sh
tiup mirror set /tmp/test-mirror-a
tiup mirror merge /tmp/test-mirror-b
tiup hello:v0.0.2 | grep TiDB

tiup uninstall
tiup uninstall tidb:v3.0.13
tiup uninstall tidb --all
tiup uninstall --all
tiup uninstall --self
