#!/bin/bash

set -eu

name=test_dm_cmd
topo=./topo/full_dm.yaml

mkdir -p ~/.tiup/bin && cp -f ./root.json ~/.tiup/bin/

tiup-dm check $topo -i ~/.ssh/id_rsa --enable-mem --enable-cpu --apply

tiup-dm --yes check $topo -i ~/.ssh/id_rsa

tiup-dm --yes deploy $name $version $topo -i ~/.ssh/id_rsa

tiup-dm list | grep "$name"

tiup-dm audit | grep "deploy $name $version"

# Get the audit id can check it just runnable
id=`tiup-dm audit | grep "deploy $name $version" | awk '{print $1}'`
tiup-dm audit $id


tiup-dm --yes start $name

# TODO: try some write operations here
tiup-dm _test $name readable

# check the data dir of tikv
tiup-dm exec $name -N 172.19.0.102 --command "grep /home/tidb/deploy/dm-master-8261/data /home/tidb/deploy/dm-master-8261/scripts/run_dm-master.sh"
tiup-dm exec $name -N 172.19.0.103 --command "grep /home/tidb/my_master_data /home/tidb/deploy/dm-master-8261/scripts/run_dm-master.sh"

tiup-dm --yes stop $name

tiup-dm --yes restart $name

# TODO: try some write operations here
tiup-dm _test $name readable

tiup-dm display $name

totol_sub_one=11

echo "start scale in dm-master"
tiup-dm --yes scale-in $name -N 172.19.0.101:8261
wait_instance_num_reach $name $totol_sub_one
echo "start scale out dm-master"
tiup-dm --yes scale-out $name ./topo/full_scale_in_dm-master.yaml

echo "start scale in dm-worker"
yes | tiup-cluster scale-in $name -N 172.19.0.102:8262
wait_instance_num_reach $name $totol_sub_one
echo "start scale out dm-worker"
yes | tiup-cluster scale-out $name ./topo/full_scale_in_dm-worker.yaml

echo "start scale in dm-portal"
yes | tiup-cluster scale-in $name -N 172.19.0.102:8280
wait_instance_num_reach $name $totol_sub_one
echo "start scale out dm-portal"
yes | tiup-cluster scale-out $name ./topo/full_scale_in_dm-worker.yaml

# TODO: try some write operations here
tiup-dm _test $name readable

tiup-cluster --yes destroy $name