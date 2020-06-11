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

function tiup-playground() {
	# echo "in function"
	if [ -f "$TEST_DIR/bin/tiup-playground.test" ]; then
	  $TEST_DIR/bin/tiup-playground.test  -test.coverprofile=$TEST_DIR/cover/cov.itest-$(date +'%s')-$RANDOM.out __DEVEL--i-heard-you-like-tests "$@"
    else
	  $TEST_DIR/../../bin/tiup-playground "$@"
	fi
}

export GO_FAILPOINTS=github.com/pingcap/tiup/components/playground/terminateEarly=return

tiup-playground v4.0.0
