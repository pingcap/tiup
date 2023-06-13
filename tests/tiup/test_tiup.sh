#!/usr/bin/env bash

set -eux

TEST_DIR=$(cd "$(dirname "$0")"; pwd)
TMP_DIR=`mktemp -d`

mkdir -p $TEST_DIR/cover

function tiup() {
    # echo "in function"
    if [ -f "$TEST_DIR/bin/tiup.test" ]; then
        $TEST_DIR/bin/tiup.test -test.coverprofile=$TEST_DIR/cover/cov.itest-$(date +'%s')-$RANDOM.out __DEVEL--i-heard-you-like-tests "$@"
    else
        $TEST_DIR/../../bin/tiup "$@"
    fi
}

rm -rf $TMP_DIR/data

# Profile home directory
mkdir -p $TMP_DIR/home/bin/
export TIUP_HOME=$TMP_DIR/home
tiup mirror set --reset

tiup list
tiup
tiup help
tiup install tidb:v5.2.2
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
CMP_TMP_DIR=`mktemp -d`
cat > $CMP_TMP_DIR/hello.sh << EOF
#! /bin/sh

echo "hello, TiDB"
EOF
chmod 755 $CMP_TMP_DIR/hello.sh
tar -C $CMP_TMP_DIR -czf $CMP_TMP_DIR/hello.tar.gz hello.sh

tiup mirror genkey

TEST_MIRROR_A=`mktemp -d`
tiup mirror init $TEST_MIRROR_A
tiup mirror set $TEST_MIRROR_A
tiup mirror grant pingcap
echo "should fail"
! tiup mirror grant pingcap # this should failed
tiup mirror publish hello v0.0.1 $CMP_TMP_DIR/hello.tar.gz hello.sh
tiup hello:v0.0.1 | grep TiDB

TEST_MIRROR_B=`mktemp -d`
tiup mirror init $TEST_MIRROR_B
tiup mirror set $TEST_MIRROR_B
tiup mirror grant pingcap
tiup mirror publish hello v0.0.2 $CMP_TMP_DIR/hello.tar.gz hello.sh
tiup mirror set $TEST_MIRROR_A
tiup mirror merge $TEST_MIRROR_B
tiup hello:v0.0.2 | grep TiDB

tiup uninstall
tiup uninstall tidb:v3.0.13
tiup uninstall tidb --all
tiup uninstall --all
tiup uninstall --self

rm -rf $TMP_DIR $CMP_TMP_DIR $TEST_MIRROR_A $TEST_MIRROR_B
