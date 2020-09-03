#!/bin/bash

set -eu

source script/cmd_subtest.sh

echo "test cluster for verision v4.0.4 wo/ TLS, via easy ssh"
cmd_subtest v4.0.4 false false
