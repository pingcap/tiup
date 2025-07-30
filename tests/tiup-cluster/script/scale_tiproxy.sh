#!/bin/bash

set -eu

function scale_tiproxy() {
    mkdir -p ~/.tiup/bin/

    version=$1
    native_ssh=$2

    common_args="--wait-timeout=360"
    if [ $native_ssh == true ]; then
        common_args="$common_args --ssh=system"
    fi

    name="test_scale_tiproxy_$RANDOM"
    topo=./topo/tiproxy.yaml

    check_cert_file="ls /home/tidb/deploy/tidb-4000/tls/tiproxy-session.crt /home/tidb/deploy/tidb-4000/tls/tiproxy-session.key"
    check_cert_config="grep -q session-token-signing-key /home/tidb/deploy/tidb-4000/conf/tidb.toml"

    tiup-cluster $common_args --yes deploy $name $version $topo -i ~/.ssh/id_rsa

    # the session certs exist
    tiup-cluster $common_args exec $name -N n1 --command "$check_cert_file"
    # the configurations are updated
    tiup-cluster $common_args exec $name -N n1 --command "$check_cert_config"

    tiup-cluster $common_args list | grep "$name"

    tiup-cluster $common_args --yes start $name

    tiup-cluster $common_args _test $name writable

    tiup-cluster $common_args display $name

    tiup-cluster $common_args --yes reload $name --skip-restart

    total_sub_one=7
    total=8

    # disable tiproxy
    echo "start scale in tiproxy"
    tiup-cluster $common_args --yes scale-in $name -N n1:6000
    wait_instance_num_reach $name $total $native_ssh

    # scale in tidb and scale out again
    echo "start scale in tidb"
    tiup-cluster $common_args --yes scale-in $name -N n2:4000
    wait_instance_num_reach $name $total_sub_one $native_ssh
    echo "start scale out tidb"
    topo=./topo/full_scale_in_tidb_2nd.yaml
    tiup-cluster $common_args --yes scale-out $name $topo
    # the session certs don't exist on the new tidb
    ! tiup-cluster $common_args exec $name -N n2 --command "$check_cert_file"
    # the configurations are not updated on the new tidb
    ! tiup-cluster $common_args exec $name -N n2 --command "$check_cert_config"

    # enable tiproxy again
    echo "start scale out tiproxy"
    topo=./topo/full_scale_in_tiproxy.yaml
    tiup-cluster $common_args --yes scale-out $name $topo
    # the session certs exist on the new tidb
    tiup-cluster $common_args exec $name -N n2 --command "$check_cert_file"
    # the configurations are updated on the new tidb
    tiup-cluster $common_args exec $name -N n2 --command "$check_cert_config"

    # scale in tidb and scale out again
    echo "start scale in tidb"
    tiup-cluster $common_args --yes scale-in $name -N n2:4000
    wait_instance_num_reach $name $total $native_ssh
    echo "start scale out tidb"
    topo=./topo/full_scale_in_tidb_2nd.yaml
    tiup-cluster $common_args --yes scale-out $name $topo
    # the session certs exist on the new tidb
    tiup-cluster $common_args exec $name -N n2 --command "$check_cert_file"
    # the configurations are updated on the new tidb
    tiup-cluster $common_args exec $name -N n2 --command "$check_cert_config"

    tiup-cluster $common_args _test $name writable
    tiup-cluster $common_args --yes destroy $name
}
