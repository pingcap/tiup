#!/bin/bash

function cmd_subtest() {
    mkdir -p ~/.tiup/bin/

    version=$1
    test_cdc=$2

    name="test_cmd_$RANDOM"
    if [ $test_cdc = true ]; then
        topo=./topo/full.yaml
    else
        topo=./topo/full_without_cdc.yaml
    fi

    tiup-cluster check $topo -i ~/.ssh/id_rsa --enable-mem --enable-cpu --apply

    tiup-cluster --yes check $topo -i ~/.ssh/id_rsa

    tiup-cluster --yes deploy $name $version $topo -i ~/.ssh/id_rsa

    tiup-cluster list | grep "$name"

    tiup-cluster audit | grep "deploy $name $version"

    # Get the audit id can check it just runnable
    id=`tiup-cluster audit | grep "deploy $name $version" | awk '{print $1}'`
    tiup-cluster audit $id

    tiup-cluster --yes start $name

    tiup-cluster _test $name writable

    # check the data dir of tikv
    tiup-cluster exec $name -N 172.19.0.102 --command "grep /home/tidb/deploy/tikv-20160/data /home/tidb/deploy/tikv-20160/scripts/run_tikv.sh"
    tiup-cluster exec $name -N 172.19.0.103 --command "grep /home/tidb/my_kv_data /home/tidb/deploy/tikv-20160/scripts/run_tikv.sh"

    tiup-cluster --yes stop $name

    tiup-cluster --yes restart $name

    tiup-cluster _test $name writable

    tiup-cluster display $name

    if [ $test_cdc = true ]; then
        totol_sub_one=19
    else
        totol_sub_one=16
    fi

    echo "start scale in tidb"
    tiup-cluster --yes scale-in $name -N 172.19.0.101:4000
    wait_instance_num_reach $name $totol_sub_one
    echo "start scale out tidb"
    tiup-cluster --yes scale-out $name ./topo/full_scale_in_tidb.yaml

    # echo "start scale in tikv"
    # tiup-cluster --yes scale-in $name -N 172.19.0.103:20160
    # wait_instance_num_reach $name $totol_sub_one
    # echo "start scale out tikv"
    # tiup-cluster --yes scale-out $name ./topo/full_scale_in_tikv.yaml

    echo "start scale in pd"
    tiup-cluster --yes scale-in $name -N 172.19.0.103:2379
    wait_instance_num_reach $name $totol_sub_one
    echo "start scale out pd"
    tiup-cluster --yes scale-out $name ./topo/full_scale_in_pd.yaml

    echo "start scale in pump"
    tiup-cluster --yes scale-in $name -N 172.19.0.103:8250
    wait_instance_num_reach $name $totol_sub_one
    echo "start scale out pump"
    tiup-cluster --yes scale-out $name ./topo/full_scale_in_pump.yaml

    if [ $test_cdc = "true" ]; then
        echo "start scale in cdc"
        yes | tiup-cluster scale-in $name -N 172.19.0.103:8300
        wait_instance_num_reach $name $totol_sub_one
        echo "start scale out cdc"
        yes | tiup-cluster scale-out $name ./topo/full_scale_in_cdc.yaml
    fi

    echo "start scale in grafana"
    tiup-cluster --yes scale-in $name -N 172.19.0.101:3000
    wait_instance_num_reach $name $totol_sub_one
    echo "start scale out grafana"
    tiup-cluster --yes scale-out $name ./topo/full_scale_in_grafana.yaml

    tiup-cluster _test $name writable

    tiup-cluster --yes destroy $name
}