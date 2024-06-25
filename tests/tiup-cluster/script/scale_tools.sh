#!/bin/bash

set -eu

function scale_tools() {
    mkdir -p ~/.tiup/bin/

    version=$1
    test_tls=$2
    native_ssh=$3

    client=""
    if [ $native_ssh == true ]; then
        client="--ssh=system"
    fi

    name="test_scale_tools_$RANDOM"
    if [ $test_tls = true ]; then
        topo=./topo/full_tls.yaml
    else
        topo=./topo/full_without_tiflash.yaml
    fi

    tiup-cluster $client --yes deploy $name $version $topo -i ~/.ssh/id_rsa

    # check the local config
    tiup-cluster $client exec $name -N n1 --command "grep magic-string-for-test /home/tidb/deploy/prometheus-9090/conf/tidb.rules.yml"
    tiup-cluster $client exec $name -N n1 --command "grep magic-string-for-test /home/tidb/deploy/grafana-3000/dashboards/tidb.json"
    tiup-cluster $client exec $name -N n1 --command "grep magic-string-for-test /home/tidb/deploy/alertmanager-9093/conf/alertmanager.yml"
    tiup-cluster $client exec $name -N n1 --command "grep alertmanagers /home/tidb/deploy/prometheus-9090/conf/prometheus.yml"
    for item in pump drainer tidb tikv pd grafana node_exporter blackbox_exporter; do
        tiup-cluster $client exec $name -N n1 --command "grep $item /home/tidb/deploy/prometheus-9090/conf/prometheus.yml"
    done

    tiup-cluster $client list | grep "$name"

    tiup-cluster $client --yes start $name

    tiup-cluster $client _test $name writable

    tiup-cluster $client display $name

    if [ $test_tls = true ]; then
        total_sub_one=19
        total=20
        total_add_one=21
    else
        total_sub_one=21
        total=22
        total_add_one=23
    fi

    echo "start scale in pump"
    tiup-cluster $client --yes scale-in $name -N n3:8250
    wait_instance_num_reach $name $total_sub_one $native_ssh
    echo "start scale out pump"
    topo=./topo/full_scale_in_pump.yaml
    tiup-cluster $client --yes scale-out $name $topo

    echo "start scale in cdc"
    yes | tiup-cluster $client scale-in $name -N n3:8300
    wait_instance_num_reach $name $total_sub_one $native_ssh
    echo "start scale out cdc"
    topo=./topo/full_scale_in_cdc.yaml
    yes | tiup-cluster $client scale-out $name $topo

    if [ $test_tls = false ]; then
        echo "start scale in tispark"
        yes | tiup-cluster $client --yes scale-in $name -N n4:7078
        wait_instance_num_reach $name $total_sub_one $native_ssh
        echo "start scale out tispark"
        topo=./topo/full_scale_in_tispark.yaml
        yes | tiup-cluster $client --yes scale-out $name $topo
    fi

    echo "start scale in grafana"
    tiup-cluster $client --yes scale-in $name -N n1:3000
    wait_instance_num_reach $name $total_sub_one $native_ssh
    echo "start scale out grafana"
    topo=./topo/full_scale_in_grafana.yaml
    tiup-cluster $client --yes scale-out $name $topo

    echo "start scale out prometheus"
    topo=./topo/full_scale_in_prometheus.yaml
    tiup-cluster $client --yes scale-out $name $topo
    wait_instance_num_reach $name $total_add_one $native_ssh
    echo "start scale in prometheus"
    tiup-cluster $client --yes scale-in $name -N n2:9090
    wait_instance_num_reach $name $total $native_ssh

    # make sure grafana dashboards has been set to default (since the full_sale_in_grafana.yaml didn't provide a local dashboards dir)
    ! tiup-cluster $client exec $name -N n1 --command "grep magic-string-for-test /home/tidb/deploy/grafana-3000/dashboards/tidb.json"

    # currently tiflash is not supported in TLS enabled cluster
    # and only Tiflash support data-dir in multipath
    if [ $test_tls = false ]; then
        echo "start scale out tiflash(first time)"
        topo=./topo/full_scale_in_tiflash.yaml
        tiup-cluster $client --yes scale-out $name $topo
        tiup-cluster $client exec $name -N n1 --command "grep tiflash /home/tidb/deploy/prometheus-9090/conf/prometheus.yml"
        # ensure scale-out will mark pd.enable-placement-rules to true. ref https://github.com/pingcap/tiup/issues/1226
        curl n3:2379/pd/api/v1/config 2>/dev/null | grep '"enable-placement-rules": "true"'

        # ensure tiflash's data dir exists
        tiup-cluster $client exec $name -N n3 --command "ls /home/tidb/deploy/tiflash-9000/data1"
        tiup-cluster $client exec $name -N n3 --command "ls /data/tiflash-data"
        echo "start scale in tiflash"
        tiup-cluster $client --yes scale-in $name -N n3:9000
        sleep 20
        tiup-cluster $client display $name | grep Tombstone
        echo "start prune tiflash"
        yes | tiup-cluster $client prune $name
        wait_instance_num_reach $name $total $native_ssh
        ! tiup-cluster $client exec $name -N n3 --command "ls /home/tidb/deploy/tiflash-9000/data1"
        ! tiup-cluster $client exec $name -N n3 --command "ls /data/tiflash-data"
        echo "start scale out tiflash(second time)"
        topo=./topo/full_scale_in_tiflash.yaml
        tiup-cluster $client --yes scale-out $name $topo
    fi

    tiup-cluster $client _test $name writable
    tiup-cluster $client --yes destroy $name

    # test cluster log dir
    tiup-cluster notfound-command 2>&1 | grep $HOME/.tiup/logs/tiup-cluster-debug
    TIUP_LOG_PATH=/tmp/a/b tiup-cluster notfound-command 2>&1 | grep /tmp/a/b/tiup-cluster-debug
}
