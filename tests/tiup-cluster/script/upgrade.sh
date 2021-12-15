#!/bin/bash

set -eu

function upgrade() {
    mkdir -p ~/.tiup/bin && cp -f ./root.json ~/.tiup/bin/

    local old_version=$1
    local version=$2
    local test_tls=$3
    local native_ssh=$4
    local proxy_ssh=$5
    local node="n"
    local client=()
    local topo_sep=""
    local name="test_upgrade_$RANDOM"

    if [ $proxy_ssh = true ]; then
        node="p"
        client+=("--ssh-proxy-host=bastion")
        topo_sep="proxy"
    fi

    if [ $test_tls = true ]; then
        topo=./topo/${topo_sep}/upgrade_tls.yaml
    else
        topo=./topo/${topo_sep}/upgrade.yaml
    fi

    if [ $native_ssh == true ]; then
        client+=("--ssh=system")
    fi

    yes | tiup-cluster "${client[@]}" deploy $name $old_version $topo -i ~/.ssh/id_rsa

    yes | tiup-cluster "${client[@]}" start $name
    # ENV_LABELS_ENV will be replaced only if the rule_dir is not specified.
    if [ $test_tls = true ]; then
        tiup-cluster "${client[@]}" exec $name -N n1 --command "grep -q ${name} /home/tidb/deploy/prometheus-9090/conf/*.rules.yml"
        ! tiup-cluster "${client[@]}" exec $name -N n1 --command "grep -q ENV_LABELS_ENV /home/tidb/deploy/prometheus-9090/conf/*.rules.yml"
    fi

    tiup-cluster "${client[@]}" _test $name writable

    yes | tiup-cluster "${client[@]}" upgrade $name $version --transfer-timeout 60

    tiup-cluster "${client[@]}" _test $name writable

    # test edit-config & reload
    # change the config of pump and check it after reload
    # https://stackoverflow.com/questions/5978108/open-vim-from-within-bash-shell-script
    EDITOR=ex tiup-cluster "${client[@]}" edit-config -y $name <<EOEX
:%s/1 mib/2 mib/g
:x
EOEX

    yes | tiup-cluster "${client[@]}" reload $name --transfer-timeout 60

    tiup-cluster "${client[@]}" exec $name -R pump --command "grep '2 mib' /home/tidb/deploy/pump-8250/conf/pump.toml"

    tiup-cluster "${client[@]}" _test $name writable

    yes | tiup-cluster "${client[@]}" destroy $name
}
