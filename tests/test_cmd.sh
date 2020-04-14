#!/bin/bash

set -eu

version=${1-v4.0.0-rc}
name=test_cmd
topo=./topo/full.yaml


yes | tiup-cluster deploy $name $version $topo -i ~/.ssh/id_rsa

yes | tiup-cluster start $name

yes | tiup-cluster stop $name

yes | tiup-cluster restart $name

tiup-cluster display $name

totol_sub_one=16

echo "start scale in tidb"
yes | tiup-cluster scale-in $name -N 172.19.0.101:4000
wait_instance_num_reach $name $totol_sub_one
echo "start scale out tidb"
yes | tiup-cluster scale-out $name ./topo/full_scale_in_tidb.yaml

echo "start scale in tikv"
yes | tiup-cluster scale-in $name -N 172.19.0.103:20160
wait_instance_num_reach $name $totol_sub_one
echo "start scale out tikv"
yes | tiup-cluster scale-out $name ./topo/full_scale_in_tikv.yaml

echo "start scale in pd"
yes | tiup-cluster scale-in $name -N 172.19.0.103:2379
wait_instance_num_reach $name $totol_sub_one
echo "start scale out pd"
yes | tiup-cluster scale-out $name ./topo/full_scale_in_pd.yaml

echo "start scale in pump"
yes | tiup-cluster scale-in $name -N 172.19.0.103:8250
wait_instance_num_reach $name $totol_sub_one
echo "start scale out pump"
yes | tiup-cluster scale-out $name ./topo/full_scale_in_pump.yaml



yes | tiup-cluster destroy $name

