#!/usr/bin/env bash

set -eux

TEST_DIR=$(cd "$(dirname "$0")"; pwd)
TMP_DIR=$TEST_DIR/_tmp


# Profile home directory
mkdir -p $TMP_DIR/home/bin/
export TIUP_HOME=$TMP_DIR/home
curl https://tiup-mirrors.pingcap.com/root.json -o $TMP_DIR/home/bin/root.json

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

# usage: check_tidb_num 1
# make sure the tidb number is 1 or other specified number
function check_tidb_num() {
    mustbe=$1
    num=$(tiup-playground display | grep "tidb" | wc -l | sed 's/ //g')
    if [ "$num" != "$mustbe" ]; then
        echo "unexpected tidb instance number: $num"
        tiup-playground display
    fi
}

function kill_all() {
    killall -9 tidb-server || true
    killall -9 tikv-server || true
    killall -9 pd-server || true
    killall -9 tiflash || true
    killall -9 grafana-server || true
    killall -9 tiup-playground || true
    killall -9 prometheus || true
    cat $outfile
}

outfile=/tmp/tiup-playground-test.out
tiup-playground v6.0.0 > $outfile 2>&1 &

# wait $outfile generated
sleep 3

trap "kill_all" EXIT

# wait start cluster successfully
timeout 300 grep -q "CLUSTER START SUCCESSFULLY" <(tail -f $outfile)

tiup-playground display | grep -qv "exit"
tiup-playground scale-out --db 2
sleep 5

# ensure prometheus/data dir exists,
# fix https://github.com/pingcap/tiup/issues/1039
ls "${TIUP_HOME}/data/test_play/prometheus/data"

# 1(init) + 2(scale-out)
check_tidb_num 3

# get pid of one tidb instance and scale-in
pid=`tiup-playground display | grep "tidb" | awk 'NR==1 {print $1}'`
tiup-playground scale-in --pid $pid

sleep 5
check_tidb_num 2

# get pid of one tidb instance and kill it
pid=`tiup-playground display | grep "tidb" | awk 'NR==1 {print $1}'`
kill -9 $pid
sleep 5

echo "*display after kill -9:"
tiup-playground display
tiup-playground display | grep "signal: killed" | wc -l | grep -q "1"

# get pid of one tidb instance and kill it
pid=`tiup-playground display | grep "tidb" | grep -v "killed" | awk 'NR==1 {print $1}'`
kill $pid
sleep 5
echo "*display after kill:"
tiup-playground display
tiup-playground display | grep -E "terminated|exit" | wc -l | grep -q "1"

killall -2 tiup-playground.test || killall -2 tiup-playground

sleep 60

# test restart with same data
tiup-playground v6.0.0 > $outfile 2>&1 &

# wait $outfile generated
sleep 3

# wait start cluster successfully
timeout 300 grep -q "CLUSTER START SUCCESSFULLY" <(tail -f $outfile)

cat $outfile | grep ":3930" | grep -q "Done"

echo -e "\033[0;36m<<< Run all test success >>>\033[0m"
