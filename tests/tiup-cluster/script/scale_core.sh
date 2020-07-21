#!/bin/bash

function scale_core() {
    mkdir -p ~/.tiup/bin/

    version=$1

    name="test_scale_core_$RANDOM"
    topo=./topo/full_without_cdc.yaml

    tiup-cluster --yes deploy $name $version $topo -i ~/.ssh/id_rsa

    tiup-cluster list | grep "$name"

    tiup-cluster --yes start $name

    tiup-cluster _test $name writable

    tiup-cluster display $name

    total_sub_one=18

    echo "start scale in tidb"
    tiup-cluster --yes scale-in $name -N 172.19.0.101:4000
    wait_instance_num_reach $name $total_sub_one
    echo "start scale out tidb"
    tiup-cluster --yes scale-out $name ./topo/full_scale_in_tidb.yaml

    # echo "start scale in tikv"
    # tiup-cluster --yes scale-in $name -N 172.19.0.103:20160
    # wait_instance_num_reach $name $total_sub_one
    # echo "start scale out tikv"
    # tiup-cluster --yes scale-out $name ./topo/full_scale_in_tikv.yaml

    echo "start scale in pd"
    tiup-cluster --yes scale-in $name -N 172.19.0.103:2379
    wait_instance_num_reach $name $total_sub_one
    echo "start scale out pd"
    tiup-cluster --yes scale-out $name ./topo/full_scale_in_pd.yaml

    tiup-cluster _test $name writable
}
