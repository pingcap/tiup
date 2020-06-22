#!/bin/bash

set -eu

source script/cmd_subtest.sh

echo "test cluster for verision v4.0.0-rc"
cmd_subtest v4.0.0-rc true

echo "test cluster for verision v4.0.0-rc.1"
cmd_subtest v4.0.0-rc.1 false
