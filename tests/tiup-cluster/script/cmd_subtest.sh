#!/bin/bash

function cmd_subtest() {
    mkdir -p ~/.tiup/bin/

    version=$1
    test_tls=$2
    native_ssh=$3
    ipprefix=${TIUP_TEST_IP_PREFIX:-"$ipprefix"}

    name="test_cmd_$RANDOM"
    if [ $test_tls = true ]; then
        topo=./topo/full_tls.yaml
    else
        topo=./topo/full.yaml
    fi
    sed "s/__IPPREFIX__/$ipprefix/g" $topo.tpl > $topo

    client=""
    if [ $native_ssh == true ]; then
        client="--native-ssh"
    fi

    tiup-cluster $client check $topo -i ~/.ssh/id_rsa --enable-mem --enable-cpu --apply

    check_result=`tiup-cluster $client --yes check $topo -i ~/.ssh/id_rsa`

    # check the check result
    echo $check_result | grep "cpu-cores"
    echo $check_result | grep "memory"
    echo $check_result | grep "os-version"
    echo $check_result | grep "selinux"
    echo $check_result | grep "service"

    # This should fail because there is no such user: tidb
    ! tiup-cluster $client --yes deploy $name $version $topo -i ~/.ssh/id_rsa --skip-create-user
    # This is a normal deploy
    tiup-cluster $client --yes deploy $name $version $topo -i ~/.ssh/id_rsa
    # Cleanup cluster meta and test --skip-create-user again, this should success
    rm -rf ~/.tiup/storage/cluster/clusters/$name
    tiup-cluster $client --yes deploy $name $version $topo -i ~/.ssh/id_rsa --skip-create-user

    # check the local config
    tiup-cluster $client exec $name -N $ipprefix.101 --command "grep magic-string-for-test /home/tidb/deploy/prometheus-9090/conf/tidb.rules.yml"
    tiup-cluster $client exec $name -N $ipprefix.101 --command "grep magic-string-for-test /home/tidb/deploy/grafana-3000/dashboards/tidb.json"
    tiup-cluster $client exec $name -N $ipprefix.101 --command "grep magic-string-for-test /home/tidb/deploy/alertmanager-9093/conf/alertmanager.yml"

    tiup-cluster $client list | grep "$name"

    tiup-cluster $client audit | grep "deploy $name $version"

    # Get the audit id can check it just runnable
    id=`tiup-cluster audit | grep "deploy $name $version" | awk '{print $1}'`
    tiup-cluster $client audit $id

    tiup-cluster $client --yes start $name

    tiup-cluster $client _test $name writable

    # check the data dir of tikv
    # it's ok to omit client type after deploy
    tiup-cluster exec $name -N $ipprefix.101 --command "grep /home/tidb/deploy/tikv-20160/data /home/tidb/deploy/tikv-20160/scripts/run_tikv.sh"
    tiup-cluster exec $name -N $ipprefix.103 --command "grep /home/tidb/my_kv_data /home/tidb/deploy/tikv-20160/scripts/run_tikv.sh"

    # test patch overwrite
    tiup-cluster $client --yes patch $name ~/.tiup/storage/cluster/packages/tidb-$version-linux-amd64.tar.gz -R tidb --overwrite
    # overwrite with the same tarball twice
    tiup-cluster $client --yes patch $name ~/.tiup/storage/cluster/packages/tidb-$version-linux-amd64.tar.gz -R tidb --overwrite

    tiup-cluster $client --yes stop $name

    tiup-cluster $client --yes restart $name

    tiup-cluster $client _test $name writable

    tiup-cluster $client _test $name data

    tiup-cluster $client display $name

    # Test rename
    tiup-cluster $client rename $name "tmp-cluster-name"
    tiup-cluster $client display "tmp-cluster-name"
    tiup-cluster $client rename "tmp-cluster-name" $name

    # Test enable & disable
    tiup-cluster $client exec $name -R tidb --command="systemctl status tidb-4000|grep 'enabled;'"
    tiup-cluster $client exec $name -R pd --command="systemctl status pd-2379|grep 'enabled;'"
    tiup-cluster $client disable $name -R tidb
    tiup-cluster $client exec $name -R tidb --command="systemctl status tidb-4000|grep 'disabled;'"
    tiup-cluster $client exec $name -R pd --command="systemctl status pd-2379|grep 'enabled;'"
    tiup-cluster $client disable $name
    tiup-cluster $client exec $name -R pd --command="systemctl status pd-2379|grep 'disabled;'"
    tiup-cluster $client enable $name
    tiup-cluster $client exec $name -R tidb --command="systemctl status tidb-4000|grep 'enabled;'"
    tiup-cluster $client exec $name -R pd --command="systemctl status pd-2379|grep 'enabled;'"

    tiup-cluster $client --yes clean $name --data --all --ignore-node $ipprefix.101:9090

    echo "checking cleanup data and log"
    tiup-cluster $client exec $name -N $ipprefix.101 --command "ls /home/tidb/deploy/prometheus-9090/log/prometheus.log"
    ! tiup-cluster $client exec $name -N $ipprefix.101 --command "ls /home/tidb/deploy/tikv-20160/log/tikv.log"

    tiup-cluster $client --yes start $name

    ! tiup-cluster $client _test $name data

    tiup-cluster $client --yes destroy $name
}
