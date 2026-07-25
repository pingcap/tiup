#!/bin/bash

set -eu
set -eE -o functrace

failure() {
    local lineno=$2
    local fn=$3
    local exitstatus=$4
    local msg=$5
    local lineno_fns=${1% 0}
    if [[ "$lineno_fns" != "0" ]] ; then
        lineno="${lineno} ${lineno_fns}"
    fi
    echo "${BASH_SOURCE[1]}:${fn}[${lineno}] Failed with status ${exitstatus}: $msg"
}
trap 'failure "${BASH_LINENO[*]}" "$LINENO" "${FUNCNAME[*]:-script}" "$?" "$BASH_COMMAND"' ERR

# instance_num <name> <use-native-ssh>
# get the instance number of the cluster
# filter the output of the go test
# PASS
# coverage: 12.7% of statements in github.com/pingcap/tiup/components/cluster/...
function instance_num() {
    name=$1
    native_ssh=$2

    client=""
    if [ $native_ssh == true ]; then
        client="--ssh=system"
    fi

    count=$(tiup-cluster $client display $name | grep "Total nodes" | awk -F ' ' '{print $3}')

    echo $count
}

# wait_instance_num_reach <name> <target_num> <use-native-ssh>
# wait the instance number of cluster reach the target_num.
# timeout 120 second
function wait_instance_num_reach() {
    name=$1
    target_num=$2
    native_ssh=$3

    client=""
    if [ $native_ssh == true ]; then
        client="--ssh=system"
    fi

    for ((i=0;i<120;i++))
    do
        tiup-cluster $client prune $name --yes
        count=$(instance_num $name $native_ssh)
        if [ "$count" == "$target_num" ]; then
            echo "instance number reach $target_num"
            return
        else
            sleep 1
        fi

        sleep 1
    done

    echo "fail to wait instance number reach $target_num, count $count, retry num: $i"
    tiup-cluster $client display $name
    exit -1
}

function assert_default_prometheus_external_labels() {
    name=$1
    node=$2
    prometheus_config=/home/tidb/deploy/prometheus-9090/conf/prometheus.yml

    tiup-cluster exec $name -N $node --command "grep -q \"cluster: '$name'\" $prometheus_config"
    tiup-cluster exec $name -N $node --command "grep -q 'monitor: \"prometheus\"' $prometheus_config"
}

function assert_prometheus_external_labels() {
    name=$1
    node=$2
    environment=$3
    region=$4
    prometheus_config=/home/tidb/deploy/prometheus-9090/conf/prometheus.yml

    assert_default_prometheus_external_labels $name $node
    tiup-cluster exec $name -N $node --command "grep -q 'environment: \"$environment\"' $prometheus_config"
    tiup-cluster exec $name -N $node --command "grep -q 'region: \"$region\"' $prometheus_config"
}

function assert_prometheus_external_label_absent() {
    name=$1
    node=$2
    label=$3
    value=$4
    prometheus_config=/home/tidb/deploy/prometheus-9090/conf/prometheus.yml
    pattern="$label: \"$value\""
    pattern_b64=$(printf '%s' "$pattern" | base64 | tr -d '\n')
    config_b64=$(printf '%s' "$prometheus_config" | base64 | tr -d '\n')
    remote_command="bash -lc 'pattern=\$(printf %s \"$pattern_b64\" | base64 -d); config=\$(printf %s \"$config_b64\" | base64 -d); grep -F -q -- \"\$pattern\" \"\$config\"; rc=\$?; echo __GREP_RC__:\$rc; exit 0'"

    set +e
    output=$(tiup-cluster exec $name -N $node --command "$remote_command" 2>&1)
    status=$?
    set -e

    if [ $status -ne 0 ]; then
        echo "$output"
        return $status
    fi

    grep_rc=$(echo "$output" | sed -n 's/.*__GREP_RC__:\([0-9][0-9]*\).*/\1/p' | tail -n 1)
    if [ -z "$grep_rc" ]; then
        echo "$output"
        echo "failed to parse grep exit code while checking Prometheus external_labels"
        return 1
    fi

    case "$grep_rc" in
        0)
            echo "$output"
            echo "expected Prometheus external label '$pattern' to be absent in $prometheus_config on $node"
            return 1
            ;;
        1)
            return 0
            ;;
        *)
            echo "$output"
            echo "grep failed with exit code $grep_rc while checking Prometheus external_labels in $prometheus_config on $node"
            return 1
            ;;
    esac
}
