#!/usr/bin/env bash

set -eux

TEST_DIR=$(cd "$(dirname "$0")"; pwd)
TMP_DIR=$TEST_DIR/_tmp

TIDB_VERSION="v6.2.0"
KVCDC_VERSION="v1.0.0"
MIRROR="https://tiup-mirrors.pingcap.com"
# MIRROR="http://staging.tiup-server.pingcap.net" # uncomment if staging environment is using.


# Profile home directory
mkdir -p $TMP_DIR/home/bin/
export TIUP_HOME=$TMP_DIR/home
curl $MIRROR/root.json -o $TMP_DIR/home/bin/root.json

# Prepare data directory
rm -rf $TIUP_HOME/data
mkdir -p $TIUP_HOME/data
export TIUP_INSTANCE_DATA_DIR=$TIUP_HOME/data/test_play
mkdir -p $TIUP_INSTANCE_DATA_DIR

mkdir -p $TEST_DIR/cover

function tiup-playground() {
    # echo "in function"
    if [ -f "$TEST_DIR/bin/tiup-playground.test" ]; then
        $TEST_DIR/bin/tiup-playground.test -test.coverprofile=$TEST_DIR/cover/cov.itest-$(date +'%s')-$RANDOM.out __DEVEL--i-heard-you-like-tests "$@"
    else
        $TEST_DIR/../../bin/tiup-playground "$@"
    fi
}

function tiup() {
    $TEST_DIR/../../bin/tiup "$@"
}

# usage: check_tidb_num 1
# make sure the TiKV-CDC number is 1 or other specified number
function check_kvcdc_num() {
    mustbe=$1
    num=$(tiup-playground display | grep "tikv-cdc" | wc -l | sed 's/ //g')
    if [ "$num" != "$mustbe" ]; then
        echo "unexpected tikv-cdc instance number: $num"
        tiup-playground display
    fi
}

function kill_all() {
    killall -9 tidb-server || true
    killall -9 tikv-server || true
    killall -9 pd-server || true
    killall -9 tikv-cdc || true
    killall -9 grafana-server || true
    killall -9 tiup-playground || true
    killall -9 prometheus || true
    killall -9 ng-monitoring-server || true
    cat $outfile
}

outfile=/tmp/tiup-playground-test.out
tiup mirror set $MIRROR
# no tiflash to speed up
tiup-playground $TIDB_VERSION --db 1 --pd 1 --kv 1 --tiflash 0 --kvcdc 1 --kvcdc.version $KVCDC_VERSION > $outfile 2>&1 &

# wait $outfile generated
sleep 3

trap "kill_all" EXIT

# wait start cluster successfully
timeout 300 grep -q "CLUSTER START SUCCESSFULLY" <(tail -f $outfile)

tiup-playground display | grep -qv "exit"
tiup-playground scale-out --kvcdc 2
sleep 5

# 1(init) + 2(scale-out)
check_kvcdc_num 3

# get pid of one tikv-cdc instance and scale-in
pid=`tiup-playground display | grep "tikv-cdc" | awk 'NR==1 {print $1}'`
tiup-playground scale-in --pid $pid

sleep 5
check_kvcdc_num 2

# exit
killall -2 tiup-playground.test || killall -2 tiup-playground
sleep 30

echo -e "\033[0;36m<<< Run all test success >>>\033[0m"
