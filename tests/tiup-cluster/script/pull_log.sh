#!/bin/bash

out_dir=$1
ipprefix=${TIUP_TEST_IP_PREFIX:-"172.19.0"}

mkdir -p $out_dir

for i in {100..105}
do
    h="${ipprefix}.${i}"
    echo $h
    mkdir -p $out_dir/$h

    if [ "$i" == "100" ]; then
        find ~/.tiup/logs -type f -name "*.log" -exec cp "{}" $out_dir/$h \;
    else
        logs=$(ssh -o "StrictHostKeyChecking no" root@$h "find /home/tidb | grep '.*log/.*\.log'")
        for log in $logs
        do
            scp -o "StrictHostKeyChecking no" -pr root@$h:$log "$out_dir/$h/"
        done
    fi
done
chmod -R 777 $out_dir
