#!/bin/bash

set -eu

source script/scale_tools.sh

echo "test scaling of tools components in cluster for version v5.3.0 w/ TLS, via easy ssh"
scale_tools v6.0.0 true false
