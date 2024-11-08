#!/usr/bin/env bash

set -eux

TEST_DIR=$(cd "$(dirname "$0")"; pwd)
TMP_DIR=$TEST_DIR/_tmp
TIDB_VERSION="v6.2.0"

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
set +x
    # echo "in function"
    if [ -f "$TEST_DIR/bin/tiup-playground.test" ]; then
        $TEST_DIR/bin/tiup-playground.test -test.coverprofile=$TEST_DIR/cover/cov.itest-$(date +'%s')-$RANDOM.out __DEVEL--i-heard-you-like-tests "$@"
    else
        $TEST_DIR/../../bin/tiup-playground "$@"
    fi
set -x
}

# usage: check_instance_num tidb 1
# make sure the tidb number is 1 or other specified number
function check_instance_num() {
    instance=$1
    mustbe=$2
    num=$(tiup-playground display | grep "$instance" | wc -l | sed 's/ //g')
    if [ "$num" != "$mustbe" ]; then
        echo "unexpected $instance instance number: $num"
        tiup-playground display
    fi
}

function kill_all() {
    killall -9 tidb-server || true
    killall -9 tikv-server || true
    killall -9 pd-server || true
    killall -9 tikv-cdc || true
    killall -9 tiflash || true
    killall -9 tiproxy || true
    killall -9 grafana-server || true
    killall -9 tiup-playground || true
    killall -9 prometheus || true
    killall -9 ng-monitoring-server || true
    cat $outfile
}

outfile=/tmp/tiup-playground-test.out
tiup-playground $TIDB_VERSION > $outfile 2>&1 &

# wait $outfile generated
sleep 3

trap "kill_all" EXIT

# wait start cluster successfully
n=0
while [ "$n" -lt 600 ] && ! grep -q "TiDB Playground Cluster is started" $outfile; do
	n=$(( n + 1 ))
	sleep 1
done
n=0
while [ "$n" -lt 10 ] && ! tiup-playground display; do
	n=$(( n + 1 ))
	sleep 1
done
tiup-playground scale-out --db 2
sleep 5

# ensure prometheus/data dir exists,
# fix https://github.com/pingcap/tiup/issues/1039
ls "${TIUP_HOME}/data/test_play/prometheus/data"

# 1(init) + 2(scale-out)
check_instance_num tidb 3

# get pid of one tidb instance and scale-in
pid=`tiup-playground display | grep "tidb" | awk 'NR==1 {print $1}'`
tiup-playground scale-in --pid $pid

sleep 5
check_instance_num tidb 2

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

sleep 100

# test restart with same data
tiup-playground $TIDB_VERSION > $outfile 2>&1 &

# wait $outfile generated
sleep 3

# wait start cluster successfully
timeout 300 grep -q "TiDB Playground Cluster is started" <(tail -f $outfile)

cat $outfile | grep ":3930" | grep -q "Done"

# start another cluster with tag
TAG="test_1"
outfile_1=/tmp/tiup-playground-test_1.out
# no TiFlash to speed up
tiup-playground $TIDB_VERSION --tag $TAG --db 2 --tiflash 0 > $outfile_1 2>&1 &
sleep 3
timeout 300 grep -q "TiDB Playground Cluster is started" <(tail -f $outfile_1)
tiup-playground --tag $TAG display | grep -qv "exit"

# TiDB scale-out to 4
tiup-playground --tag $TAG scale-out --db 2
sleep 5
# TiDB scale-in to 3
pid=`tiup-playground --tag $TAG display | grep "tidb" | awk 'NR==1 {print $1}'`
tiup-playground --tag $TAG scale-in --pid $pid
sleep 5
# check number of TiDB instances.
tidb_num=$(tiup-playground --tag $TAG display | grep "tidb" | wc -l | sed 's/ //g')
if [ "$tidb_num" != 3 ]; then
    echo "unexpected tidb instance number: $tidb_num"
    exit 1
fi

killall -2 tiup-playground.test || killall -2 tiup-playground
sleep 100

# test for TiKV-CDC
echo -e "\033[0;36m<<< Run TiKV-CDC test >>>\033[0m"
tiup-playground $TIDB_VERSION --db 1 --pd 1 --kv 1 --tiflash 0 --kvcdc 1 --kvcdc.version v1.0.0 > $outfile 2>&1 &
sleep 3
timeout 300 grep -q "TiDB Playground Cluster is started" <(tail -f $outfile)
tiup-playground display | grep -qv "exit"
# scale out
tiup-playground scale-out --kvcdc 2
sleep 5
check_instance_num tikv-cdc 3 # 1(init) + 2(scale-out)
# scale in
pid=`tiup-playground display | grep "tikv-cdc" | awk 'NR==1 {print $1}'`
tiup-playground scale-in --pid $pid
sleep 5
check_instance_num tikv-cdc 2

# exit all
killall -2 tiup-playground.test || killall -2 tiup-playground
sleep 30

# test for TiProxy
echo -e "\033[0;36m<<< Run TiProxy test >>>\033[0m"
tiup-playground $TIDB_VERSION --db 1 --pd 1 --kv 1 --tiflash 0 --tiproxy 1 --tiproxy.version "nightly" > $outfile 2>&1 &
sleep 3
timeout 300 grep -q "TiDB Playground Cluster is started" <(tail -f $outfile)
tiup-playground display | grep -qv "exit"
# scale out
tiup-playground scale-out --tiproxy 1
sleep 5
check_instance_num tiproxy 2
# scale in
pid=`tiup-playground display | grep "tiproxy" | awk 'NR==1 {print $1}'`
tiup-playground scale-in --pid $pid
sleep 5
check_instance_num tiproxy 1

# exit all
killall -2 tiup-playground.test || killall -2 tiup-playground
sleep 30

echo -e "\033[0;36m<<< Run all test success >>>\033[0m"
