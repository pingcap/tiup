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

# instance_num <name>
# get the instance number of the dm
# filter the output of the go test
# PASS
# coverage: 12.7% of statements in github.com/pingcap/tiup/components/dm/...
function instance_num() {
    name=$1

    count=$(tiup-dm display $name | grep "Total nodes" | awk -F ' ' '{print $3}')

    echo $count
}

# wait_instance_num_reach <name> <target_num>
# wait the instance number of dm reach the target_num.
# timeout 120 second
function wait_instance_num_reach() {
    name=$1
    target_num=$2

    for ((i=0;i<120;i++))
    do
        tiup-dm prune $name --yes
        count=$(instance_num $name)
        if [ "$count" == "$target_num" ]; then
            echo "instance number reach $target_num"
            return
        else
            sleep 1
        fi

        sleep 1
    done

    echo "fail to wait instance number reach $target_num, count $count, retry num: $i"
    tiup-dm display $name
    exit -1
}
