#!/bin/bash


out_dir=$1

mkdir -p $out_dir

hosts="172.19.0.101 172.19.0.102 172.19.0.103 172.19.0.104 172.19.0.105"

for h in $hosts
do
	echo $h
	mkdir $out_dir/$h

	logs=$(ssh -o "StrictHostKeyChecking no" root@$h "find /home/tidb | grep '.*log/.*\.log'")

	for log in $logs
	do
		scp -o "StrictHostKeyChecking no" -r root@$h:$log "$out_dir/$h/"
	done
done






