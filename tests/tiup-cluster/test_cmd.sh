#!/bin/bash

set -eu

source script/cmd_subtest.sh

echo "test cluster for verision v4.0.0"
cmd_subtest v4.0.0 true

echo "test cluster for verision v4.0.2"
cmd_subtest v4.0.2 false
