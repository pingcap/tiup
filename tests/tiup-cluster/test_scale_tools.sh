#!/bin/bash

set -eu

source script/scale_tools.sh

echo "test scaling of tools components in cluster for version v6.2.0, via easy ssh"
scale_tools v6.2.0 false false
