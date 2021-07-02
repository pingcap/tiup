#!/bin/bash

set -eu

function scale_core() {
    mkdir -p ~/.tiup/bin/

    local version=$1
    local test_tls=$2
    local native_ssh=$3
    local proxy_ssh=$4
    local node="n"
    local client=()
    local topo_sep=""
    local name="test_scale_core_$RANDOM"
    local ssh_cmd=(ssh -o "StrictHostKeyChecking=no" -o "PasswordAuthentication=no")

    if [ $proxy_ssh = true ]; then
        node="p"
        topo_sep="proxy"
        client+=("--ssh-proxy-host=bastion")
        ssh_cmd+=(-o "ProxyCommand=ssh bastion -W %h:%p")
    fi

    if [ $test_tls = true ]; then
        topo=./topo/${topo_sep}/full_tls.yaml
    else
        topo=./topo/${topo_sep}/full.yaml
    fi

    if [ $native_ssh == true ]; then
        client+=("--ssh=system")
    fi

    tiup-cluster "${client[@]}" --yes deploy $name $version $topo -i ~/.ssh/id_rsa

    tiup-cluster "${client[@]}" list | grep "$name"

    tiup-cluster "${client[@]}" --yes start $name

    tiup-cluster "${client[@]}" _test $name writable

    tiup-cluster "${client[@]}" display $name

    tiup-cluster "${client[@]}" --yes reload $name --skip-restart

    if [ $test_tls = true ]; then
        total_sub_one=18
    else
        total_sub_one=21
    fi

    echo "start scale in tidb"
    tiup-cluster "${client[@]}" --yes scale-in $name -N ${node}1:4000
    wait_instance_num_reach $name $total_sub_one $native_ssh $proxy_ssh
    # ensure Prometheus's configuration is updated automatically
    ! tiup-cluster "${client[@]}" exec $name -N ${node}1 --command "grep -q ${node}1:10080 /home/tidb/deploy/prometheus-9090/conf/prometheus.yml"

    echo "start scale out tidb"
    topo=./topo/${topo_sep}/full_scale_in_tidb.yaml
    tiup-cluster "${client[@]}" --yes scale-out $name $topo
    # after scale-out, ensure the service is enabled
    tiup-cluster "${client[@]}" exec $name -N ${node}1 --command "systemctl status tidb-4000 | grep Loaded |grep 'enabled; vendor'"
    tiup-cluster "${client[@]}" exec $name -N ${node}1 --command "grep -q ${node}1:10080 /home/tidb/deploy/prometheus-9090/conf/prometheus.yml"

    # scale in tikv maybe exists in several minutes or hours, and the GitHub CI is not guaranteed
    # echo "start scale in tikv"
    # tiup-cluster --yes scale-in $name -N ${node}3:20160
    # wait_instance_num_reach $name $total_sub_one $native_ssh $proxy_ssh
    # echo "start scale out tikv"
    # topo=./topo/${topo_sep}/full_scale_in_tikv.yaml
    # tiup-cluster --yes scale-out $name $topo

    echo "start scale in pump"
    tiup-cluster "${client[@]}" --yes scale-in $name -N ${node}3:8250
    wait_instance_num_reach $name $total_sub_one $native_ssh $proxy_ssh

    # ensure Prometheus's configuration is updated automatically
    ! tiup-cluster "${client[@]}" exec $name -N ${node}1 --command "grep -q ${node}3:8250 /home/tidb/deploy/prometheus-9090/conf/prometheus.yml"

    echo "start scale out pump"
    topo=./topo/${topo_sep}/full_scale_in_pump.yaml
    tiup-cluster "${client[@]}" --yes scale-out $name $topo
    # after scale-out, ensure this instance come back
    tiup-cluster "${client[@]}" exec $name -N ${node}1 --command "grep -q ${node}3:8250 /home/tidb/deploy/prometheus-9090/conf/prometheus.yml"

    echo "start scale in pd"
    tiup-cluster "${client[@]}" --yes scale-in $name -N ${node}3:2379
    wait_instance_num_reach $name $total_sub_one $native_ssh $proxy_ssh

    # validate https://github.com/pingcap/tiup/issues/786
    # ensure that this instance is removed from the startup scripts of other components that need to rely on PD
    ! tiup-cluster "${client[@]}" exec $name -N ${node}1 --command "grep -q ${node}3:2379 /home/tidb/deploy/tidb-4000/scripts/run_tidb.sh"
    # ensure Prometheus's configuration is updated automatically
    ! tiup-cluster "${client[@]}" exec $name -N ${node}1 --command "grep -q ${node}3:2379 /home/tidb/deploy/prometheus-9090/conf/prometheus.yml"

    echo "start scale out pd"
    topo=./topo/${topo_sep}/full_scale_in_pd.yaml
    tiup-cluster "${client[@]}" --yes scale-out $name $topo
    # after scale-out, ensure this instance come back
    tiup-cluster "${client[@]}" exec $name -N ${node}1 --command "grep -q ${node}3:2379 /home/tidb/deploy/tidb-4000/scripts/run_tidb.sh"
    tiup-cluster "${client[@]}" exec $name -N ${node}1 --command "grep -q ${node}3:2379 /home/tidb/deploy/prometheus-9090/conf/prometheus.yml"

    echo "start scale in tidb"
    tiup-cluster "${client[@]}" --yes scale-in $name -N ${node}2:4000
    wait_instance_num_reach $name $total_sub_one $native_ssh $proxy_ssh
    ! tiup-cluster "${client[@]}" exec $name -N ${node}2 --command "ls /home/tidb/deploy/monitor-9100/deploy/monitor-9100"
    ! tiup-cluster "${client[@]}" exec $name -N ${node}2 --command "ps aux | grep node_exporter | grep -qv grep"
    ! tiup-cluster "${client[@]}" exec $name -N ${node}2 --command "ps aux | grep blackbox_exporter | grep -qv grep"
    # after all components on the node were scale-ined, the SSH public is automatically deleted
    ! "${ssh_cmd[@]}" -i ~/.tiup/storage/cluster/$name/ssh/id_rsa tidb@${node}2 "ls"

    echo "start scale out tidb"
    topo=./topo/${topo_sep}/full_scale_in_tidb_2nd.yaml
    tiup-cluster "${client[@]}" --yes scale-out $name $topo
    # after scalue-out, ensure node_exporter and blackbox_exporter come back
    tiup-cluster "${client[@]}" exec $name -N ${node}2 --command "ls /home/tidb/deploy/monitor-9100/deploy/monitor-9100"
    tiup-cluster "${client[@]}" exec $name -N ${node}2 --command "ps aux | grep node_exporter | grep -qv grep"
    tiup-cluster "${client[@]}" exec $name -N ${node}2 --command "ps aux | grep blackbox_exporter | grep -qv grep"

    tiup-cluster "${client[@]}" _test $name writable
    tiup-cluster "${client[@]}" --yes destroy $name
}
