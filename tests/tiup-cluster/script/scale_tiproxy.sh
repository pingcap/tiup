#!/bin/bash

set -eu

function scale_tiproxy() {
    mkdir -p ~/.tiup/bin/

    version=$1
    test_tls=$2
    native_ssh=$3

    client=""
    if [ $native_ssh == true ]; then
        client="--ssh=system"
    fi

    name="test_scale_tiproxy_$RANDOM"
    if [ $test_tls = true ]; then
        topo=./topo/full_tls.yaml
    else
        topo=./topo/full.yaml
    fi

    check_cert_file="ls /home/tidb/deploy/tidb-4000/tls/tiproxy-session.crt /home/tidb/deploy/tidb-4000/tls/tiproxy-session.key"
    check_cert_config="grep -q session-token-signing-key /home/tidb/deploy/tidb-4000/conf/tidb.toml"

    tiup-cluster $client --yes deploy $name $version $topo -i ~/.ssh/id_rsa

    # the session certs exist
    tiup-cluster $client exec $name -N n1 --command "$check_cert_file"
    # the configurations are updated
    tiup-cluster $client exec $name -N n1 --command "$check_cert_config"

    tiup-cluster $client list | grep "$name"

    tiup-cluster $client --yes start $name

    tiup-cluster $client _test $name writable

    tiup-cluster $client display $name

    tiup-cluster $client --yes reload $name --skip-restart

    if [ $test_tls = true ]; then
        total_sub_one=19
        total=20
    else
        total_sub_one=24
        total=25
    fi

    # disable tiproxy
    echo "start scale in tiproxy"
    tiup-cluster $client --yes scale-in $name -N n1:6000
    wait_instance_num_reach $name $total_sub_one $native_ssh

    echo "start scale out tidb"
    topo=./topo/full_scale_in_tidb_2nd.yaml
    tiup-cluster $client --yes scale-out $name $topo
    # the session certs doesn't exist on the new tidb
    ! tiup-cluster $client exec $name -N n2 --command "$check_cert_file"
    # the configurations are not updated on the new tidb
    ! tiup-cluster $client exec $name -N n2 --command "$check_cert_config"

    # enable tiproxy again
    echo "start scale out tiproxy"
    topo=./topo/full_scale_in_tiproxy.yaml
    tiup-cluster $client --yes scale-out $name $topo
    # the session certs exist on the new tidb
    tiup-cluster $client exec $name -N n2 --command "$check_cert_file"
    # the configurations are updated on the new tidb
    tiup-cluster $client exec $name -N n2 --command "$check_cert_config"

    # scale in tidb and scale out again
    echo "start scale in tidb"
    tiup-cluster $client --yes scale-in $name -N n2:4000
    wait_instance_num_reach $name $all $native_ssh
    echo "start scale out tidb"
    topo=./topo/full_scale_in_tidb_2nd.yaml
    tiup-cluster $client --yes scale-out $name $topo
    # the session certs exist on the new tidb
    tiup-cluster $client exec $name -N n2 --command "$check_cert_file"
    # the configurations are updated on the new tidb
    tiup-cluster $client exec $name -N n2 --command "$check_cert_config"
}