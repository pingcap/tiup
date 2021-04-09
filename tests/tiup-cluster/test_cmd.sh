#!/bin/bash

set -eu

source script/cmd_subtest.sh

echo "test cluster for verision v4.0.12 wo/ TLS, via easy ssh"
cmd_subtest 4.0.12 false false
