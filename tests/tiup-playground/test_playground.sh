#!/usr/bin/env bash

set -eux

TEST_DIR=$(cd "$(dirname "$0")"; pwd)
TMP_DIR=$TEST_DIR/_tmp
TIDB_VERSION="v8.5.0"
OS=$(uname -s)
ARCH=$(uname -m)

DEFAULT_TIFLASH=1
if [ "$OS" = "Darwin" ]; then
    # TiFlash may fail on macOS due to rlimit/core settings.
    DEFAULT_TIFLASH=0
fi

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

function wait_log_contains() {
    file=$1
    pattern=$2
    timeout_sec=$3

    n=0
    while [ "$n" -lt "$timeout_sec" ] && ! grep -Fq "$pattern" "$file"; do
        n=$(( n + 1 ))
        sleep 1
    done
    grep -Fq "$pattern" "$file"
}

# usage: check_instance_num tidb 1
# make sure the tidb number is 1 or other specified number
function check_instance_num() {
    instance=$1
    mustbe=$2
    num=$(tiup-playground display | awk -v svc="$instance" '$2==svc {n++} END {print n+0}')
    if [ "$num" != "$mustbe" ]; then
        echo "unexpected $instance instance number: $num"
        tiup-playground display
    fi
}

function first_instance_name() {
    service=$1
    shift
    tiup-playground "$@" display | awk -v svc="$service" '$2==svc {print $1; exit}'
}

function first_running_instance_pid() {
    service=$1
    shift
    tiup-playground "$@" display -v | awk -v svc="$service" '$2==svc && $5=="running" {print $7; exit}'
}

function running_instance_num() {
    service=$1
    shift
    tiup-playground "$@" display | awk -v svc="$service" '$2==svc && $4=="running" {n++} END {print n+0}'
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

function stop_playground() {
    if killall -2 tiup-playground.test 2>/dev/null; then
        return 0
    fi
    if killall -2 tiup-playground 2>/dev/null; then
        return 0
    fi
    return 1
}

outfile=/tmp/tiup-playground-test.out
tiup-playground $TIDB_VERSION --tiflash $DEFAULT_TIFLASH > $outfile 2>&1 &

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
ls "${TIUP_HOME}/data/test_play/prometheus-0/data"

# 1(init) + 2(scale-out)
check_instance_num tidb 3

# scale-in should reject specifying both --name and --pid
name=$(first_instance_name tidb)
pid=$(first_running_instance_pid tidb)
# if tiup-playground scale-in --name "$name" --pid "$pid"; then
#     echo "expected scale-in to fail when both --name and --pid are provided"
#     exit 1
# fi

# get pid of one tidb instance and scale-in
tiup-playground scale-in --pid $pid

sleep 5
check_instance_num tidb 2

# get pid of one tidb instance and kill it
before_running=$(running_instance_num tidb)
pid=$(first_running_instance_pid tidb)
kill -9 $pid
sleep 5

echo "*display after kill -9:"
tiup-playground display
after_running=$(running_instance_num tidb)
test "$after_running" -lt "$before_running"

# get pid of one tidb instance and kill it
before_running=$(running_instance_num tidb)
pid=$(first_running_instance_pid tidb)
kill $pid
sleep 5
echo "*display after kill:"
if tiup-playground display; then
    after_running=$(running_instance_num tidb)
    test "$after_running" -lt "$before_running"
else
    # Killing the last TiDB instance triggers playground auto-stop.
    test "$before_running" -eq 1
fi

if stop_playground; then
    sleep 15
fi

# test restart with same data
tiup-playground $TIDB_VERSION --tiflash $DEFAULT_TIFLASH > $outfile 2>&1 &

# wait $outfile generated
sleep 3

# wait start cluster successfully
wait_log_contains "$outfile" "TiDB Playground Cluster is started" 300

# start another cluster with tag
TAG="test_1"
outfile_1=/tmp/tiup-playground-test_1.out
# no TiFlash to speed up
tiup-playground $TIDB_VERSION --tag $TAG --db 2 --tiflash 0 > $outfile_1 2>&1 &
sleep 3
wait_log_contains "$outfile_1" "TiDB Playground Cluster is started" 300
tiup-playground --tag $TAG display | grep -qv "exit"

# TiDB scale-out to 4
tiup-playground --tag $TAG scale-out --service tidb --count 2
sleep 5
# TiDB scale-in to 3
name=$(first_instance_name tidb --tag $TAG)
tiup-playground --tag $TAG scale-in --name $name
sleep 5
# check number of TiDB instances.
tidb_num=$(tiup-playground --tag $TAG display | grep "tidb" | wc -l | sed 's/ //g')
if [ "$tidb_num" != 3 ]; then
    echo "unexpected tidb instance number: $tidb_num"
    exit 1
fi

if stop_playground; then
    sleep 15
fi

if [ "$OS" = "Linux" ]; then
    # test for TiKV-CDC
    echo -e "\033[0;36m<<< Run TiKV-CDC test >>>\033[0m"
    tiup-playground $TIDB_VERSION --db 1 --pd 1 --kv 1 --tiflash 0 --kvcdc 1 --kvcdc.version v1.0.0 > $outfile 2>&1 &
    sleep 3
    wait_log_contains "$outfile" "TiDB Playground Cluster is started" 300
    tiup-playground display | grep -qv "exit"
    # scale out
    tiup-playground scale-out --kvcdc 2
    sleep 5
    check_instance_num tikv-cdc 3 # 1(init) + 2(scale-out)
    # scale in
    name=$(first_instance_name tikv-cdc)
    tiup-playground scale-in --name $name
    sleep 5
    check_instance_num tikv-cdc 2

    # exit all
    if stop_playground; then
        sleep 30
    fi
else
    echo "skip TiKV-CDC test on ${OS}/${ARCH}"
fi

if [ "$OS" = "Linux" ]; then
    # test for TiProxy
    echo -e "\033[0;36m<<< Run TiProxy test >>>\033[0m"
    tiup-playground $TIDB_VERSION --db 1 --pd 1 --kv 1 --tiflash 0 --tiproxy 1 --tiproxy.version "nightly" > $outfile 2>&1 &
    sleep 3
    wait_log_contains "$outfile" "TiDB Playground Cluster is started" 300
    tiup-playground display | grep -qv "exit"
    # scale out
    tiup-playground scale-out --tiproxy 1
    sleep 5
    check_instance_num tiproxy 2
    # scale in
    name=$(first_instance_name tiproxy)
    tiup-playground scale-in --name $name
    sleep 5
    check_instance_num tiproxy 1

    # exit all
    if stop_playground; then
        sleep 30
    fi
else
    echo "skip TiProxy test on ${OS}/${ARCH}"
fi

echo -e "\033[0;36m<<< Run all test success >>>\033[0m"
