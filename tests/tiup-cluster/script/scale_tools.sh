#!/bin/bash

function scale_tools() {
    mkdir -p ~/.tiup/bin/

    version=$1
    native_ssh=$2

    client=""
    if [ $native_ssh == true ]; then
        client="--native-ssh"
    fi

    name="test_scale_tools_$RANDOM"
    topo=./topo/full.yaml

    tiup-cluster $client --yes deploy $name $version $topo -i ~/.ssh/id_rsa

    # check the local config
    tiup-cluster $client exec $name -N 172.19.0.101 --command "grep magic-string-for-test /home/tidb/deploy/prometheus-9090/conf/tidb.rules.yml"
    tiup-cluster $client exec $name -N 172.19.0.101 --command "grep magic-string-for-test /home/tidb/deploy/grafana-3000/dashboards/tidb.json"
    tiup-cluster $client exec $name -N 172.19.0.101 --command "grep magic-string-for-test /home/tidb/deploy/alertmanager-9093/conf/alertmanager.yml"

    tiup-cluster $client list | grep "$name"

    tiup-cluster $client --yes start $name

    tiup-cluster $client _test $name writable

    tiup-cluster $client display $name

    total_sub_one=21

    echo "start scale in pump"
    tiup-cluster $client --yes scale-in $name -N 172.19.0.103:8250
    wait_instance_num_reach $name $total_sub_one $native_ssh
    echo "start scale out pump"
    tiup-cluster $client --yes scale-out $name ./topo/full_scale_in_pump.yaml

    echo "start scale in cdc"
    yes | tiup-cluster $client scale-in $name -N 172.19.0.103:8300
    wait_instance_num_reach $name $total_sub_one $native_ssh
    echo "start scale out cdc"
    yes | tiup-cluster $client scale-out $name ./topo/full_scale_in_cdc.yaml

    echo "start scale in tispark"
    yes | tiup-cluster $client --yes scale-in $name -N 172.19.0.104:7078
    wait_instance_num_reach $name $total_sub_one $native_ssh
    echo "start scale out tispark"
    yes | tiup-cluster $client --yes scale-out $name ./topo/full_scale_in_tispark.yaml

    echo "start scale in grafana"
    tiup-cluster $client --yes scale-in $name -N 172.19.0.101:3000
    wait_instance_num_reach $name $total_sub_one $native_ssh
    echo "start scale out grafana"
    tiup-cluster $client --yes scale-out $name ./topo/full_scale_in_grafana.yaml

    # make sure grafana dashboards has been set to default (since the full_sale_in_grafana.yaml didn't provide a local dashboards dir)
    ! tiup-cluster $client exec $name -N 172.19.0.101 --command "grep magic-string-for-test /home/tidb/deploy/grafana-3000/dashboards/tidb.json"

    tiup-cluster $client _test $name writable
}
