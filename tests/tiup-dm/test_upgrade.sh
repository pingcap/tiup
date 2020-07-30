#!/bin/bash

set -eu

version=${version-nightly}
old_version=${old_version-nightly}
name=test_upgrade
topo=./topo/full_dm.yaml

mkdir -p ~/.tiup/bin && cp -f ./root.json ~/.tiup/bin/

yes | tiup-dm deploy $name $old_version $topo -i ~/.ssh/id_rsa

yes | tiup-dm start $name

# tiup-dm _test $name writable

yes | tiup-dm upgrade $name $version --transfer-timeout 60


# test edit-config & reload
# change the config of master and check it after reload
# https://stackoverflow.com/questions/5978108/open-vim-from-within-bash-shell-script
EDITOR=ex tiup-dm edit-config -y $name <<EOEX
:%s/30s/31s/g
:x
EOEX

yes | tiup-dm reload $name

# just check one instance for verify.
tiup-dm exec $name -N "172.19.0.104:8261" --command "grep '31s' /home/tidb/deploy/dm-master-8261/conf/dm-master.toml"


# test create a task and can replicate data
./script/task/run.sh


yes | tiup-dm destroy $name
