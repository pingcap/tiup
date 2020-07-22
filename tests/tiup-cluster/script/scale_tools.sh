#!/bin/bash

function scale_tools() {
    mkdir -p ~/.tiup/bin/

    version=$1

    name="test_scale_tools_$RANDOM"
    topo=./topo/full.yaml

    tiup-cluster --yes deploy $name $version $topo -i ~/.ssh/id_rsa

    tiup-cluster list | grep "$name"

    tiup-cluster --yes start $name

    tiup-cluster _test $name writable

    tiup-cluster display $name

    total_sub_one=21

    echo "start scale in pump"
    tiup-cluster --yes scale-in $name -N 172.19.0.103:8250
    wait_instance_num_reach $name $total_sub_one
    echo "start scale out pump"
    tiup-cluster --yes scale-out $name ./topo/full_scale_in_pump.yaml

    echo "start scale in cdc"
    yes | tiup-cluster scale-in $name -N 172.19.0.103:8300
    wait_instance_num_reach $name $total_sub_one
    echo "start scale out cdc"
    yes | tiup-cluster scale-out $name ./topo/full_scale_in_cdc.yaml

    echo "start scale in tispark"
    yes | tiup-cluster --yes scale-in $name -N 172.19.0.104:7078
    wait_instance_num_reach $name $total_sub_one
    echo "start scale out tispark"
    yes | tiup-cluster --yes scale-out $name ./topo/full_scale_in_tispark.yaml

    echo "start scale in grafana"
    tiup-cluster --yes scale-in $name -N 172.19.0.101:3000
    wait_instance_num_reach $name $total_sub_one
    echo "start scale out grafana"
    tiup-cluster --yes scale-out $name ./topo/full_scale_in_grafana.yaml

    tiup-cluster _test $name writable
}
