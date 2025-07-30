#!/bin/bash

set -eu

name=test_cmd
topo=./topo/full_dm.yaml

ipprefix=${TIUP_TEST_IP_PREFIX:-"172.19.0"}
sed "s/__IPPREFIX__/$ipprefix/g" $topo.tpl > $topo

mkdir -p ~/.tiup/bin && cp -f ./root.json ~/.tiup/bin/

# tiup-dm check $topo -i ~/.ssh/id_rsa --enable-mem --enable-cpu --apply

# tiup-dm --yes check $topo -i ~/.ssh/id_rsa

tiup-dm --yes deploy $name $version $topo -i ~/.ssh/id_rsa

# topology doesn't contains the section `monitored` will not deploy node_exporter, blackbox_exporter
has_exporter=0
tiup-dm exec $name -N $ipprefix.101 --command "ls /etc/systemd/system/{node,blackbox}_exporter-*.service" || has_exporter=1
if [[ $has_exporter -eq 0 ]]; then
  echo "monitoring agents should not be deployed for dm cluster if \"monitored\" section is not set."
  exit 1;
fi
tiup-dm list | grep "$name"

# debug https://github.com/pingcap/tiup/issues/666
echo "debug audit:"
ls -l ~/.tiup/storage/dm/audit/*
head -1 ~/.tiup/storage/dm/audit/*
tiup-dm audit
echo "end debug audit"

tiup-dm audit | grep "deploy $name $version"

# Get the audit id can check it just runnable
id=`tiup-dm audit | grep "deploy $name $version" | awk '{print $1}'`
tiup-dm audit $id

# check the local config
tiup-dm exec $name -N $ipprefix.101 --command "grep magic-string-for-test /home/tidb/deploy/prometheus-9090/conf/dm_worker.rules.yml"
tiup-dm exec $name -N $ipprefix.101 --command "grep magic-string-for-test /home/tidb/deploy/grafana-3000/dashboards/*.json"
tiup-dm exec $name -N $ipprefix.101 --command "grep magic-string-for-test /home/tidb/deploy/alertmanager-9093/conf/alertmanager.yml"

tiup-dm --yes start $name


# check the data dir of dm-master
tiup-dm exec $name -N $ipprefix.102 --command "grep /home/tidb/deploy/dm-master-8261/data /home/tidb/deploy/dm-master-8261/scripts/run_dm-master.sh"
tiup-dm exec $name -N $ipprefix.103 --command "grep /home/tidb/my_master_data /home/tidb/deploy/dm-master-8261/scripts/run_dm-master.sh"

# check the service enabled
tiup-dm exec $name -N $ipprefix.102 --command "systemctl status dm-master-8261 | grep 'enabled;'"
tiup-dm exec $name -N $ipprefix.102 --command "systemctl status dm-worker-8262 | grep 'enabled;'"

# check enable/disable service
tiup-dm disable $name -R dm-master
tiup-dm exec $name -N $ipprefix.102 --command "systemctl status dm-master-8261 | grep 'disabled;'"
tiup-dm exec $name -N $ipprefix.102 --command "systemctl status dm-worker-8262 | grep 'enabled;'"
tiup-dm enable $name -R dm-master
tiup-dm exec $name -N $ipprefix.102 --command "systemctl status dm-master-8261 | grep 'enabled;'"
tiup-dm exec $name -N $ipprefix.102 --command "systemctl status dm-worker-8262 | grep 'enabled;'"

tiup-dm --yes stop $name

tiup-dm --yes restart $name

tiup-dm display $name
tiup-dm display $name --uptime

total_sub_one=12

echo "start scale in dm-master"
tiup-dm --yes scale-in $name -N $ipprefix.101:8261
wait_instance_num_reach $name $total_sub_one false
# ensure Prometheus's configuration is updated automatically
! tiup-dm exec $name -N $ipprefix.101 --command "grep -q $ipprefix.101:8261 /home/tidb/deploy/prometheus-9090/conf/prometheus.yml"

echo "start scale out dm-master"
topo_master=./topo/full_scale_in_dm-master.yaml
sed "s/__IPPREFIX__/$ipprefix/g" $topo_master.tpl > $topo_master
tiup-dm --yes scale-out $name $topo_master
tiup-dm exec $name -N $ipprefix.101 --command "grep -q $ipprefix.101:8261 /home/tidb/deploy/prometheus-9090/conf/prometheus.yml"
tiup-dm exec $name -N $ipprefix.101 --command "systemctl status dm-master-8261 | grep 'enabled;'"

echo "start scale in dm-worker"
yes | tiup-dm scale-in $name -N $ipprefix.102:8262
wait_instance_num_reach $name $total_sub_one
# ensure Prometheus's configuration is updated automatically
! tiup-dm exec $name -N $ipprefix.101 --command "grep -q $ipprefix.102:8262 /home/tidb/deploy/prometheus-9090/conf/prometheus.yml"

echo "start scale out dm-worker"
topo_worker=./topo/full_scale_in_dm-worker.yaml
sed "s/__IPPREFIX__/$ipprefix/g" $topo_worker.tpl > $topo_worker
yes | tiup-dm scale-out $name $topo_worker
tiup-dm exec $name -N $ipprefix.101 --command "grep -q $ipprefix.102:8262 /home/tidb/deploy/prometheus-9090/conf/prometheus.yml"
tiup-dm exec $name -N $ipprefix.101 --command "systemctl status dm-worker-8262 | grep 'enabled;'"

echo "start scale in grafana"
yes | tiup-dm scale-in $name -N $ipprefix.101:3000
wait_instance_num_reach $name $total_sub_one

echo "start scale out grafana"
topo_grafana=./topo/full_scale_in_grafana.yaml
sed "s/__IPPREFIX__/$ipprefix/g" $topo_grafana.tpl > $topo_grafana
yes | tiup-dm scale-out $name $topo_grafana

# test grafana config
tiup-dm exec $name -N $ipprefix.101 --command "ls /home/tidb/deploy/grafana-3000/dashboards/*.json && ! grep magic-string-for-test /home/tidb/deploy/grafana-3000/dashboards/*.json"

# test create a task and can replicate data
./script/task/run.sh

# test dm log dir
tiup-dm notfound-command 2>&1 | grep $HOME/.tiup/logs/tiup-dm-debug
TIUP_LOG_PATH=/tmp/a/b tiup-dm notfound-command 2>&1 | grep /tmp/a/b/tiup-dm-debug

cp ~/.tiup/storage/dm/clusters/$name/ssh/id_rsa "/tmp/$name.id_rsa"
tiup-dm --yes destroy $name

# after destroy the cluster, the public key should be deleted
! ssh -o "StrictHostKeyChecking=no" -o "PasswordAuthentication=no" -i "/tmp/$name.id_rsa" tidb@$ipprefix.102 "ls"
unlink "/tmp/$name.id_rsa"

topo=./topo/full_dm_monitored.yaml

ipprefix=${TIUP_TEST_IP_PREFIX:-"172.19.0"}
sed "s/__IPPREFIX__/$ipprefix/g" $topo.tpl > $topo
tiup-dm --yes deploy $name $version $topo -i ~/.ssh/id_rsa

# topology contains the section `monitored` will deploy node_exporter, blackbox_exporter
tiup-dm exec $name -N $ipprefix.101 --command "ls /etc/systemd/system/{node,blackbox}_exporter-*.service"

tiup-dm --yes destroy $name
