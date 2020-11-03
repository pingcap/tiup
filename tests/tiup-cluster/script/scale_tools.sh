#!/bin/bash

function scale_tools() {
    mkdir -p ~/.tiup/bin/

    version=$1
    test_tls=$2
    native_ssh=$3
    ipprefix=${TIUP_TEST_IP_PREFIX:-"172.19.0"}

    client=""
    if [ $native_ssh == true ]; then
        client="--native-ssh"
    fi

    name="test_scale_tools_$RANDOM"
    if [ $test_tls = true ]; then
        topo=./topo/full_tls.yaml
    else
        topo=./topo/full.yaml
    fi
    sed "s/__IPPREFIX__/$ipprefix/g" $topo.tpl > $topo

    tiup-cluster $client --yes deploy $name $version $topo -i ~/.ssh/id_rsa

    # check the local config
    tiup-cluster $client exec $name -N $ipprefix.101 --command "grep magic-string-for-test /home/tidb/deploy/prometheus-9090/conf/tidb.rules.yml"
    tiup-cluster $client exec $name -N $ipprefix.101 --command "grep magic-string-for-test /home/tidb/deploy/grafana-3000/dashboards/tidb.json"
    tiup-cluster $client exec $name -N $ipprefix.101 --command "grep magic-string-for-test /home/tidb/deploy/alertmanager-9093/conf/alertmanager.yml"

    tiup-cluster $client list | grep "$name"

    tiup-cluster $client --yes start $name

    tiup-cluster $client _test $name writable

    tiup-cluster $client display $name

    if [ $test_tls = true ]; then
        total_sub_one=18
    else
        total_sub_one=21
    fi

    echo "start scale in pump"
    tiup-cluster $client --yes scale-in $name -N $ipprefix.103:8250
    wait_instance_num_reach $name $total_sub_one $native_ssh
    echo "start scale out pump"
    topo=./topo/full_scale_in_pump.yaml
    sed "s/__IPPREFIX__/$ipprefix/g" $topo.tpl > $topo
    tiup-cluster $client --yes scale-out $name $topo

    echo "start scale in cdc"
    yes | tiup-cluster $client scale-in $name -N $ipprefix.103:8300
    wait_instance_num_reach $name $total_sub_one $native_ssh
    echo "start scale out cdc"
    topo=./topo/full_scale_in_cdc.yaml
    sed "s/__IPPREFIX__/$ipprefix/g" $topo.tpl > $topo
    yes | tiup-cluster $client scale-out $name $topo

    if [ $test_tls = false ]; then
        echo "start scale in tispark"
        yes | tiup-cluster $client --yes scale-in $name -N $ipprefix.104:7078
        wait_instance_num_reach $name $total_sub_one $native_ssh
        echo "start scale out tispark"
        topo=./topo/full_scale_in_tispark.yaml
        sed "s/__IPPREFIX__/$ipprefix/g" $topo.tpl > $topo
        yes | tiup-cluster $client --yes scale-out $name $topo
    fi

    echo "start scale in grafana"
    tiup-cluster $client --yes scale-in $name -N $ipprefix.101:3000
    wait_instance_num_reach $name $total_sub_one $native_ssh
    echo "start scale out grafana"
    topo=./topo/full_scale_in_grafana.yaml
    sed "s/__IPPREFIX__/$ipprefix/g" $topo.tpl > $topo
    tiup-cluster $client --yes scale-out $name $topo

    # make sure grafana dashboards has been set to default (since the full_sale_in_grafana.yaml didn't provide a local dashboards dir)
    ! tiup-cluster $client exec $name -N $ipprefix.101 --command "grep magic-string-for-test /home/tidb/deploy/grafana-3000/dashboards/tidb.json"

    # currently tiflash is not supported in TLS enabled cluster
    # and only Tiflash support data-dir in multipath
    if [ $test_tls = false ]; then
        # ensure tiflash's data dir exists
        tiup-cluster $client exec $name -N $ipprefix.103 --command "ls /home/tidb/deploy/tiflash-9000/data1"
        tiup-cluster $client exec $name -N $ipprefix.103 --command "ls /data/tiflash-data"
        echo "start scale in tiflash"
        tiup-cluster $client --yes scale-in $name -N $ipprefix.103:9000
        tiup-cluster $client display $name | grep Tombstone
        echo "start prune tiflash"
        yes | tiup-cluster $client prune $name
        wait_instance_num_reach $name $total_sub_one $native_ssh
        ! tiup-cluster $client exec $name -N $ipprefix.103 --command "ls /home/tidb/deploy/tiflash-9000/data1"
        ! tiup-cluster $client exec $name -N $ipprefix.103 --command "ls /data/tiflash-data"
        echo "start scale out tiflash"
        topo=./topo/full_scale_in_tiflash.yaml
        sed "s/__IPPREFIX__/$ipprefix/g" $topo.tpl > $topo
        tiup-cluster $client --yes scale-out $name $topo
    fi

    tiup-cluster $client _test $name writable
}
