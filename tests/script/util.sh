#!/bin/bash

set -eu

# instance_num <name>
# get the instance number of the cluster
# filter the output of the go test
# PASS
# coverage: 12.7% of statements in github.com/pingcap-incubator/tiup-cluster/...
function instance_num() {
	name=$1
	line=$(tiup-cluster display $name | grep -v "PASS" | grep -v "coverage" | wc -l)
	count=`expr $line - 4`

	echo $count
}

# wait_instance_num_reach <name> <target_num>
# wait the instance number of cluster reach the target_num.
# timeout 120 second
function wait_instance_num_reach() {
	name=$1
	target_num=$2

	for ((i=0;i<120;i++))
	do
		count=$(instance_num $name)
		if [ "$count" == "$target_num" ]; then
			echo "instance number reach $target_num"
			return
		else
			sleep 1
		fi

		sleep 1
	done

	echo "fail to wait instance number reach $target_num, retry num: $i"
	tiup-cluster display $name
	exit -1
}
