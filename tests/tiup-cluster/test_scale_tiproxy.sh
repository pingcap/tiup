#!/bin/bash

set -eu

source script/scale_tiproxy.sh

echo "test scaling of tidb and tiproxy in cluster for version v8.2.0, via easy ssh"
scale_tiproxy v8.2.0 false
