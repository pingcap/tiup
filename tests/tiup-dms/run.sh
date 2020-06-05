#!/bin/bash

set -eu

# Change directory to the source directory of this script. Taken from:
# https://stackoverflow.com/a/246128/3858681
pushd "$( cd "$( dirname "${BASH_SOURCE[0]}"  )" >/dev/null 2>&1 && pwd  )"

function tiup-dm() {
	# echo "in function"
	if [ -f "./bin/tiup-dm.test" ]; then
	  ./bin/tiup-dms.test  -test.coverprofile=./cover/cov.itest-$(date +'%s')-$RANDOM.out __DEVEL--i-heard-you-like-tests "$@"
    else
	  ../bin/tiup-dms "$@"
	fi
}

# use run.sh test_cmd test_upgrade to run specify cases
do_cases=$*

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
