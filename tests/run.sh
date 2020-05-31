#!/bin/bash
#
# Copyright 2020 PingCAP, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# See the License for the specific language governing permissions and
# limitations under the License.

# Initialize the directory variables
TEST_DIR=$(cd "$(dirname "$0")"; pwd)
TMP_DIR=$TEST_DIR/_tmp
TIUP_HOME=$TEST_DIR/tiup_home
TIUP_MIRRORS=$TEST_DIR/tiup_mirrors
TIUP_EXPECTED=$TEST_DIR/expected
PATH=$TIUP_HOME/bin:$PATH

mkdir -p "$TMP_DIR"
rm -rf "$TIUP_HOME/manifest"
rm -rf "$TIUP_HOME/components"
rm -rf "$TIUP_HOME/data"

if command -v tput &>/dev/null && tty -s; then
  RED=$(tput setaf 1)
  GREEN=$(tput setaf 2)
  MAGENTA=$(tput setaf 5)
  NORMAL=$(tput sgr0)
  BOLD=$(tput bold)
else
  RED=$(echo -en "\e[31m")
  GREEN=$(echo -en "\e[32m")
  MAGENTA=$(echo -en "\e[35m")
  NORMAL=$(echo -en "\e[00m")
  BOLD=$(echo -en "\e[01m")
fi

echo "TEST DIRECTORY:   ${BOLD} $TEST_DIR ${NORMAL}"
echo "TIUP BINARY PATH: ${BOLD} $(command -v tiup) ${NORMAL}"
echo "TEMP DIRECTORY:   ${BOLD} $TMP_DIR ${NORMAL}"
echo "TIUP HOME DIR:    ${BOLD} $TIUP_HOME ${NORMAL}"
echo "TIUP MIRRORS:     ${BOLD} $TIUP_MIRRORS ${NORMAL}"
echo "EXPTECTED RESULT: ${BOLD} $TIUP_EXPECTED ${NORMAL}"

cd "$TIUP_MIRRORS"
TIUP_HOME=$TIUP_HOME TIUP_MIRRORS=$TIUP_MIRRORS TEST_DIR=$TEST_DIR sh prepare.sh
cd "$TEST_DIR"

success=0
failed=0

for file in "cases_cov.json" "cases.json"
do
  case=$(jq < "$TEST_DIR/$file" '. | length')
  for index in $(seq 1 $case)
  do
    echo ""

    cmd=$(jq < "$TEST_DIR/$file" -r ".[$index-1]|.command")
    path=$(jq < "$TEST_DIR/$file" -r ".[$index-1]|.path")
    cov="$TEST_DIR/../cover/cov.integration-test.$file-$index.out"

    echo "Coverage file: $cov for cmd: '$cmd'"

    if [ "$file" = "cases_cov.json" ]; then
      cmd="${cmd[@]:0:4} -test.coverprofile=$cov DEVEL ${cmd[@]:4}"
    fi

    if [ "$path" = "" ]; then
      echo "${MAGENTA}✔ Directly output case: cmd='$cmd' ${NORMAL}"
      TIUP_HOME=$TIUP_HOME TIUP_MIRRORS=$TIUP_MIRRORS $cmd
      continue
    fi

    # delete the last two lines
    mkdir -p $(dirname "$TMP_DIR/$path")
    actual=$(TIUP_HOME=$TIUP_HOME TIUP_MIRRORS=$TIUP_MIRRORS $cmd \
    | sed "s+${TIUP_MIRRORS}+TIUP_MIRRORS_INTEGRATION_TEST+" \
    | sed "s+${TIUP_HOME}+TIUP_HOME_INTEGRATION_TEST+")

    # delete coverage output
    case "$actual" in
      *coverage:*)
        echo "$actual" | tail -n 2
        echo "$actual" | sed -e "$ d"  | sed -e "$ d" > "$TMP_DIR/$path"
        ;;
      *)
        echo "$actual" > "$TMP_DIR/$path"
        ;;
    esac

    actual=$(cat "$TMP_DIR/$path")
    expected=$(cat "$TIUP_EXPECTED/$path")

    if [ "$expected" != "$actual" ]; then
      echo "${RED}✖ Failed case: cmd='$cmd'${NORMAL}"
      echo " + expected path:   ${BOLD} $TIUP_EXPECTED/$path ${NORMAL}"
      echo " + actual got path: ${BOLD} $TMP_DIR/$path ${NORMAL}"

      failed=$((failed+1))

      echo "${BOLD}-----------------------------------DIFF START-----------------------------------------${NORMAL}" >&2
      diff --ignore-space-change -B -E \
       --old-group-format="${RED}%<${NORMAL}" \
       --new-group-format="${GREEN}%>${NORMAL}" \
       --unchanged-group-format="" \
       "$TIUP_EXPECTED/$path" "$TMP_DIR/$path"
      echo "${BOLD}-----------------------------------DIFF END-------------------------------------------${NORMAL}" >&2
    else
      success=$((success+1))
      echo "${MAGENTA}✔ Passed case: cmd='$cmd' ${NORMAL}"
    fi
  done
done

# Run playground integration test in github action environment
if [ $GITHUB_ACTION ]; then
  GO_FAILPOINTS=github.com/pingcap-incubator/tiup/components/playground/terminateEarly=return \
   TIUP_HOME=$TIUP_HOME TIUP_MIRRORS=$TIUP_MIRRORS \
   tiup -T test-playground playground \
   -test.coverprofile="$TEST_DIR/../cover/cov.integration-test.playground.out" v4.0.0-rc --monitor

  echo "${BOLD}SUMMARY: total case: $((success+failed)), success: $success, failed: $failed${NORMAL}"

  if [ ! -f "$TIUP_HOME/data/test-playground/dsn" ]; then
    echo "${RED}✖ Failed run playground${NORMAL}"
  fi
fi

# Clean data
rm -rf "$TIUP_HOME"
rm -rf testmirrors

if [ $failed -gt 0 ]; then
  exit 1
fi
