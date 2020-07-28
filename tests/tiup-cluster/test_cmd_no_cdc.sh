#!/bin/bash

set -eu

source script/cmd_subtest.sh

echo "test cluster for verision v4.0.2 without CDC, via easy ssh"
cmd_subtest v4.0.2 false false
