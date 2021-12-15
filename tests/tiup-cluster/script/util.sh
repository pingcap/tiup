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
    local name=$1
    local native_ssh=$2
    local proxy_ssh=$3
    local client=()

    if [ $native_ssh == true ]; then
        client+=("--ssh=system")
    fi
    if [ $proxy_ssh == true ]; then
        client+=("--ssh-proxy-host=bastion")
    fi

    count=$(tiup-cluster "${client[@]}" display $name | grep "Total nodes" | awk -F ' ' '{print $3}')

    echo $count
}

# wait_instance_num_reach <name> <target_num> <use-native-ssh>
# wait the instance number of cluster reach the target_num.
# timeout 120 second
function wait_instance_num_reach() {
    local name=$1
    local target_num=$2
    local native_ssh=$3
    local proxy_ssh=$4
    local client=()
    if [ $native_ssh == true ]; then
        client+=("--ssh=system")
    fi
    if [ $proxy_ssh == true ]; then
        client+=("--ssh-proxy-host=bastion")
    fi

    for ((i=0;i<120;i++))
    do
        tiup-cluster "${client[@]}" prune $name --yes
        count=$(instance_num $name $native_ssh $proxy_ssh)
        if [ "$count" == "$target_num" ]; then
            echo "instance number reach $target_num"
            return
        else
            sleep 1
        fi

        sleep 1
    done

    echo "fail to wait instance number reach $target_num, count $count, retry num: $i"
    tiup-cluster "${client[@]}" display $name
    exit -1
}
