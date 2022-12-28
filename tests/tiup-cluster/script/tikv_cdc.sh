#!/bin/bash

set -eu

function tikv_cdc_test() {
	mkdir -p ~/.tiup/bin/

	version="nightly"
	topo_name="tikv_cdc"
	test_tls=false
	tikv_cdc_patch=""

	while [[ $# -gt 0 ]]; do
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
		--tikv-cdc-patch)
			tikv_cdc_patch="$2"
			shift
			shift
			;;
		esac
	done

	name="test_tikv_cdc_$RANDOM"
	if [ $test_tls = true ]; then
		topo=./topo/${topo_name}_tls.yaml
	else
		topo=./topo/$topo_name.yaml
	fi

	# identify SSH via ssh-agent
	eval $(ssh-agent)
	ssh-add /root/.ssh/id_rsa

	tiup-cluster check $topo --apply

	# Test check version. Cluster version >= v6.2.0 is required.
	# Error message: "Error: init config failed: n3:8600: tikv-cdc only supports cluster version v6.2.0 or later"
	! tiup-cluster --yes deploy $name 6.1.0 $topo

	tiup-cluster --yes deploy $name $version $topo

	# check the local config
	tiup-cluster exec $name -R tikv-cdc --command 'grep "gc-ttl = 43200$" /home/tidb/deploy/tikv-cdc-8600/conf/tikv-cdc.toml'

	tiup-cluster list | grep "$name"

	tiup-cluster audit | grep "deploy $name $version"

	# Get the audit id can check it just runnable
	id=$(tiup-cluster audit | grep "deploy $name $version" | awk '{print $1}')
	tiup-cluster audit $id

	tiup-cluster --yes start $name

	# Patch
	if [[ ! -z "$tikv_cdc_patch" ]]; then
		wget https://tiup-mirrors.pingcap.com/tikv-cdc-v${tikv_cdc_patch}-linux-amd64.tar.gz
		tiup install tikv-cdc:v${tikv_cdc_patch}
		tiup-cluster --yes patch $name ./tikv-cdc-v${tikv_cdc_patch}-linux-amd64.tar.gz -R tikv-cdc --offline
		tiup-cluster display $name | grep "tikv-cdc (patched)"
	fi

	tiup-cluster _test $name writable

	# check the data dir
	tiup-cluster exec $name -N n3 --command "grep /home/tidb/deploy/tikv-cdc-8600/data /home/tidb/deploy/tikv-cdc-8600/scripts/run_tikv-cdc.sh"
	tiup-cluster exec $name -N n4 --command "grep /home/tidb/tikv_cdc_data /home/tidb/deploy/tikv-cdc-8600/scripts/run_tikv-cdc.sh"

	# test patch overwrite
	if [[ ! -z "$tikv_cdc_patch" ]]; then
		tiup-cluster --yes patch $name ./tikv-cdc-v${tikv_cdc_patch}-linux-amd64.tar.gz -R tikv-cdc --overwrite
		# overwrite with the same tarball twice
		tiup-cluster --yes patch $name ./tikv-cdc-v${tikv_cdc_patch}-linux-amd64.tar.gz -R tikv-cdc --overwrite
	fi

	tiup-cluster --yes stop $name

	tiup-cluster --yes start $name -R pd,tikv-cdc

	tiup-cluster --yes restart $name

	tiup-cluster _test $name writable

	tiup-cluster _test $name data

	# Test enable & disable
	tiup-cluster exec $name -R tikv-cdc --command="systemctl status tikv-cdc-8600|grep 'enabled;'"
	tiup-cluster disable $name -R tikv-cdc
	tiup-cluster exec $name -R tikv-cdc --command="systemctl status tikv-cdc-8600|grep 'disabled;'"
	tiup-cluster disable $name
	tiup-cluster enable $name
	tiup-cluster exec $name -R tikv-cdc --command="systemctl status tikv-cdc-8600|grep 'enabled;'"

	tiup-cluster --yes clean $name --data --all --ignore-node n5:8600

	echo "checking cleanup data and log"
	! tiup-cluster exec $name -N n3 --command "ls /home/tidb/deploy/tikv-cdc-8600/log/tikv.log"

	tiup-cluster --yes start $name

	! tiup-cluster _test $name data

	tiup-cluster --yes destroy $name
}

function tikv_cdc_scale_test() {
	mkdir -p ~/.tiup/bin/

	version="nightly"
	topo_name="tikv_cdc"
	test_tls=false

	while [[ $# -gt 0 ]]; do
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
		esac
	done

	name=test_tikv_cdc_scale_$RANDOM
	if [ $test_tls = true ]; then
		topo=./topo/${topo_name}_tls.yaml
	else
		topo=./topo/${topo_name}.yaml
	fi

	tiup-cluster --yes deploy $name $version $topo

	tiup-cluster --yes start $name

	tiup-cluster _test $name writable

	tiup-cluster display $name

	total_sub_one=13
	total=14
	total_add_one=15

	echo -e "\033[0;36m Start scale in tikv-cdc (-n3) \033[0m"
	yes | tiup-cluster scale-in $name -N n3:8600
	wait_instance_num_reach $name $total_sub_one false

	echo -e "\033[0;36m Start scale out tikv-cdc (+n5) \033[0m"
	mkdir -p /tmp/topo
	topo=/tmp/topo/tikv_cdc_scale_in.yaml
	cat <<EOF > $topo
kvcdc_servers:
  - host: n5
EOF
	yes | tiup-cluster scale-out $name $topo
	wait_instance_num_reach $name $total false

	echo -e "\033[0;36m Scale out another tikv-cdc on n5 to verify port conflict detection \033[0m"
	cat <<EOF > $topo
kvcdc_servers:
  - host: n5
    data_dir: "/home/tidb/tikv_cdc_data_1"
EOF
	# should fail with message "Error: port conflict for '8600' between 'kvcdc_servers:n5.port' and 'kvcdc_servers:n5.port'"
	! yes | tiup-cluster scale-out $name $topo # should fail

	echo -e "\033[0;36m Scale out another tikv-cdc on n5 with different port & data_dir \033[0m"
	cat <<EOF > $topo
kvcdc_servers:
  - host: n5
    port: 8666
    data_dir: "/home/tidb/tikv_cdc_data_1"
EOF
	yes | tiup-cluster scale-out $name $topo
	wait_instance_num_reach $name $total_add_one false

	# scale in n4, as n4 should be the owner.
	echo -e "\033[0;36m Start scale in tikv-cdc (-n4) \033[0m"
	yes | tiup-cluster scale-in $name -N n4:8600
	wait_instance_num_reach $name $total false

	tiup-cluster _test $name writable

	tiup-cluster --yes destroy $name
}
