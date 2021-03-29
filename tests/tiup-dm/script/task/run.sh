#!/bin/bash

# ./up.sh --dev --compose ./docker-compose.dm.yml
# deploy a cluster using 'tests/tiup-dm/topo/full_dm.yaml'
# run this script in control node to prepare some data and create the task.


set -eu

pushd "$( cd "$( dirname "${BASH_SOURCE[0]}"  )" >/dev/null 2>&1 && pwd  )"

ctl="tiup dmctl:nightly"
wd=$(pwd)

$ctl --master-addr n1:8261 operate-source create $wd/source1.yml
$ctl --master-addr n1:8261 operate-source create $wd/source2.yml

cat $wd/db1.prepare.sql | mysql -h mysql1
cat $wd/db2.prepare.sql | mysql -h mysql2

# the task will replicate data into db_target.t_target of tidb1:4000
echo "drop table if exists db_target.t_target;" | mysql -h tidb1 -P 4000
$ctl --master-addr n1:8261 start-task $wd/task.yaml

# check data
# should has 9 row in the prepare data, one for the header here.
sleep 10 # wait to replicate
line=$(echo "select * from db_target.t_target" | mysql -h tidb1 -P 4000 | wc -l)
if [ $line = 10 ];then
    echo "replicate data success"
else
    echo "fail to replicate data, line is $line"
    exit -1
fi
