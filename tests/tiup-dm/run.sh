#!/bin/bash

set -eu

# Change directory to the source directory of this script. Taken from:
# https://stackoverflow.com/a/246128/3858681
pushd "$( cd "$( dirname "${BASH_SOURCE[0]}"  )" >/dev/null 2>&1 && pwd  )"

# use run.sh --do-cases "test_cmd test_upgrade" to run specify cases
do_cases=""

while [[ $# -gt 0 ]]
do
	key="$1"
	case $key in
		--native-ssh)
			echo "run using native ssh"
			export TIUP_NATIVE_SSH=true
			export GO_FAILPOINTS='github.com/pingcap/tiup/pkg/cluster/executor/assertNativeSSH=return(true)'
			shift # past argument
			;;
		--do-cases)
			do_cases="$2"
			shift # past argument
			shift # past value
			;;
		*)
			shift #
	esac
done

PATH=$PATH:/tiup-cluster/bin
export TIUP_CLUSTER_PROGRESS_REFRESH_RATE=10s
export TIUP_CLUSTER_EXECUTE_DEFAULT_TIMEOUT=300s

export version=${version-nightly}

function tiup-dm() {
	mkdir -p ~/.tiup/bin && cp -f ./root.json ~/.tiup/bin/
	# echo "in function"
	if [ -f "./bin/tiup-dm.test" ]; then
	  ./bin/tiup-dm.test  -test.coverprofile=./cover/cov.itest-$(date +'%s')-$RANDOM.out __DEVEL--i-heard-you-like-tests "$@"
    else
	  ../../bin/tiup-dm "$@"
	fi
}

. ./script/util.sh

if [  "$do_cases" == "" ]; then
  for script in ./test_*.sh; do
    echo "run test: $script"
    . $script
  done
else
  for script in "${do_cases[@]}"; do
    echo "run test: $script.sh"
    . ./$script.sh
  done
fi

echo "\033[0;36m<<< Run all test success >>>\033[0m"
