#!/bin/bash

set -eu

function cmd_subtest() {
    mkdir -p ~/.tiup/bin/

    version="nightly"
    topo_name="full"
    test_tls=false
    native_ssh=false
    
    while [[ $# -gt 0 ]]
    do
        case $1 in
            --version)
                version="$2"
                shift
                shift
                ;;
            --topo)
                topo_name="$2"
                shift
                shift
                ;;
            --tls)
                test_tls=true
                shift
                ;;
            --native-ssh)
                native_ssh=true
                shift
                ;;
        esac
    done

    name="test_cmd_$RANDOM"
    if [ $test_tls = true ]; then
        topo=./topo/${topo_name}_tls.yaml
    else
        topo=./topo/${topo_name}.yaml
    fi

    client=""
    if [ $native_ssh == true ]; then
        client="--ssh=system"
    fi

    # identify SSH via ssh-agent
    eval $(ssh-agent)
    ssh-add /root/.ssh/id_rsa

    mv /root/.ssh/id_rsa{,.bak}
    tiup-cluster $client check $topo -i ~/.ssh/id_rsa --enable-mem --enable-cpu --apply
    mv /root/.ssh/id_rsa{.bak,}

    check_result=`tiup-cluster $client --yes check $topo -i ~/.ssh/id_rsa`

    # check the check result
    echo $check_result | grep "cpu-cores"
    echo $check_result | grep "memory"
    echo $check_result | grep "os-version"
    echo $check_result | grep "selinux"
    echo $check_result | grep "service"
    echo $check_result | grep "thp"

    for i in {1..5}; do ssh -o "StrictHostKeyChecking=no" -o "PasswordAuthentication=no" n"$i" "grep -q tidb /etc/passwd && (killall -u tidb; userdel -f -r tidb) || true"; done
    # This should fail because there is no such user: tidb
    ! tiup-cluster $client --yes deploy $name $version $topo -i ~/.ssh/id_rsa --skip-create-user
    # This is a normal deploy
    tiup-cluster $client --yes deploy $name $version $topo -i ~/.ssh/id_rsa
    # Cleanup cluster meta and test --skip-create-user again, this should success
    rm -rf ~/.tiup/storage/cluster/clusters/$name
    tiup-cluster $client --yes deploy $name $version $topo -i ~/.ssh/id_rsa --skip-create-user

    # check the local config
    tiup-cluster $client exec $name -N n1 --command "grep tidb.rules.yml /home/tidb/deploy/prometheus-9090/conf/prometheus.yml"
    ! tiup-cluster $client exec $name -N n1 --command "grep node.rules.yml /home/tidb/deploy/prometheus-9090/conf/prometheus.yml"
    tiup-cluster $client exec $name -N n1 --command "grep magic-string-for-test /home/tidb/deploy/prometheus-9090/conf/tidb.rules.yml"
    tiup-cluster $client exec $name -N n1 --command "grep magic-string-for-test /home/tidb/deploy/grafana-3000/dashboards/tidb.json"
    tiup-cluster $client exec $name -N n1 --command "grep magic-string-for-test /home/tidb/deploy/alertmanager-9093/conf/alertmanager.yml"

    tiup-cluster $client list | grep "$name"

    tiup-cluster $client audit | grep "deploy $name $version"

    # Get the audit id can check it just runnable
    id=`tiup-cluster audit | grep "deploy $name $version" | awk '{print $1}'`
    tiup-cluster $client audit $id

    tiup-cluster $client --yes start $name

    # Patch a stopped cluster
    tiup-cluster $client --yes patch $name ~/.tiup/storage/cluster/packages/tidb-v$version-linux-amd64.tar.gz -R tidb --offline
    tiup-cluster $client display $name | grep "tidb (patched)"

    tiup-cluster $client _test $name writable

    # check the data dir of tikv
    # it's ok to omit client type after deploy
    tiup-cluster exec $name -N n1 --command "grep /home/tidb/deploy/tikv-20160/data /home/tidb/deploy/tikv-20160/scripts/run_tikv.sh"
    tiup-cluster exec $name -N n1 --command "grep advertise-status-addr /home/tidb/deploy/tikv-20160/scripts/run_tikv.sh"
    tiup-cluster exec $name -N n3 --command "grep /home/tidb/my_kv_data /home/tidb/deploy/tikv-20160/scripts/run_tikv.sh"

    # test checkpoint
    tiup-cluster exec $name -N n1 --command "touch /tmp/checkpoint"
    tiup-cluster exec $name -N n1 --command "ls /tmp/checkpoint"
    tiup-cluster exec $name -N n1 --command "rm -f /tmp/checkpoint"
    id=`tiup-cluster audit | grep "exec $name" | grep "ls /tmp/checkpoint" | awk '{print $1}'`
    tiup-cluster replay --yes $id
    ! tiup-cluster exec $name -N n1 --command "ls /tmp/checkpoint"

    # test patch overwrite
    tiup-cluster $client --yes patch $name ~/.tiup/storage/cluster/packages/tidb-v$version-linux-amd64.tar.gz -R tidb --overwrite
    # overwrite with the same tarball twice
    tiup-cluster $client --yes patch $name ~/.tiup/storage/cluster/packages/tidb-v$version-linux-amd64.tar.gz -R tidb --overwrite
    # test patch with a non-executable entry
    rm -rf tidb-server
    touch tidb-server # this is a non-executable regular file
    tar -czf tidb-non-executable.tar.gz tidb-server
    ! tiup-cluster $client --yes patch $name ./tidb-non-executable.tar.gz -R tidb
    # test patch with a dir entry
    rm -rf tidb-server
    mkdir tidb-server
    tar -czf tidb-dir-entry.tar.gz tidb-server
    ! tiup-cluster $client --yes patch $name ./tidb-dir-entry.tar.gz -R tidb

    tiup-cluster $client --yes stop $name

    # test start prometheus,grafana won't hang-forever(can't update topology)
    # let the CI to stop the job if hang forever
    ! tiup-cluster $client --yes start $name -R prometheus,grafana

    tiup-cluster $client --yes restart $name

    tiup-cluster $client _test $name writable

    tiup-cluster $client _test $name data

    display_result=`tiup-cluster $client display $name`
    echo "$display_result" | grep "Cluster type"
    echo "$display_result" | grep "Cluster name"
    echo "$display_result" | grep "Cluster version"
    echo "$display_result" | grep "Dashboard URL"
    echo "$display_result" | grep "Total nodes"
    echo "$display_result" | grep -v "Since"

    # display with --uptime should show process uptime
    display_result=`tiup-cluster $client display $name --uptime`
    echo "$display_result" | grep "Since"

    # Test rename
    tiup-cluster $client --yes rename $name "tmp-cluster-name"
    tiup-cluster $client display "tmp-cluster-name"
    tiup-cluster $client --yes rename "tmp-cluster-name" $name

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

    tiup-cluster $client --yes clean $name --data --all --ignore-node n1:9090

    # Test push and pull
    echo "test_transfer $name $RANDOM `date`" > test_transfer_1.txt
    tiup-cluster $client push $name test_transfer_1.txt "{{ .DeployDir }}/test_transfer.txt" -R grafana
    tiup-cluster $client pull $name "{{ .DeployDir }}/test_transfer.txt" test_transfer_2.txt -R grafana
    diff test_transfer_1.txt test_transfer_2.txt
    rm -f test_transfer_{1,2}.txt

    echo "checking cleanup data and log"
    tiup-cluster $client exec $name -N n1 --command "ls /home/tidb/deploy/prometheus-9090/log/prometheus.log"
    ! tiup-cluster $client exec $name -N n1 --command "ls /home/tidb/deploy/tikv-20160/log/tikv.log"

    tiup-cluster $client --yes start $name

    ! tiup-cluster $client _test $name data

    cp ~/.tiup/storage/cluster/clusters/$name/ssh/id_rsa "/tmp/$name.id_rsa"
    tiup-cluster $client --yes destroy $name

    # after destroy the cluster, the public key should be deleted
    ! ssh -o "StrictHostKeyChecking=no" -o "PasswordAuthentication=no" -i "/tmp/$name.id_rsa" tidb@n1 "ls"
    unlink "/tmp/$name.id_rsa"
}
