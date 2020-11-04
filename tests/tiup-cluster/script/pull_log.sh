#!/bin/bash


out_dir=$1

ipprefix=${TIUP_TEST_IP_PREFIX:-"172.19.0"}

mkdir -p $out_dir

for i in {100..105}
do
    h="${ipprefix}.${i}"
    echo $h
    mkdir -p $out_dir/$h

    if [ "$h" == "${ipprefix}.100" ]
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
