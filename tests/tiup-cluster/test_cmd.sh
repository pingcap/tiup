#!/bin/bash

set -eu

source script/cmd_subtest.sh

echo "test cluster for version v4.0.12 wo/ TLS, via easy ssh"
cmd_subtest --version 4.0.12
