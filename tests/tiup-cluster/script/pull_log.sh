#!/bin/bash


out_dir=$1

mkdir -p $out_dir

hosts="172.19.0.100 172.19.0.101 172.19.0.102 172.19.0.103 172.19.0.104 172.19.0.105"

for h in $hosts
do
	echo $h
	mkdir $out_dir/$h

	if [ "$h" == "172.19.0.100" ]
	then
  	logs=$(ssh -o "StrictHostKeyChecking no" root@$h "find /tiup-cluster/logs | grep '.*log/.*\.log'")
	else
  	logs=$(ssh -o "StrictHostKeyChecking no" root@$h "find /home/tidb | grep '.*log/.*\.log'")
  fi


	for log in $logs
	do
		scp -o "StrictHostKeyChecking no" -r root@$h:$log "$out_dir/$h/"
	done
done






