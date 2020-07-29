#!/bin/bash

set -eu

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
        client="--native-ssh"
    fi

	line=$(tiup-cluster $client display $name | grep -v "PASS" | grep -v "coverage" | wc -l)
	count=`expr $line - 4`

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
        client="--native-ssh"
    fi

	for ((i=0;i<120;i++))
	do
		count=$(instance_num $name $native_ssh)
		if [ "$count" == "$target_num" ]; then
			echo "instance number reach $target_num"
			return
		else
			sleep 1
		fi

		sleep 1
	done

	echo "fail to wait instance number reach $target_num, retry num: $i"
	tiup-cluster $client display $name
	exit -1
}
