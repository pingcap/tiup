#!/bin/bash

set -eu

source script/cmd_subtest.sh

echo "test cluster for version v6.2.0 wo/ TLS, via easy ssh"
cmd_subtest --version 6.2.0
