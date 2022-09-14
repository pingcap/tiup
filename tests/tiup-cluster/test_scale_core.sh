#!/bin/bash

set -eu

source script/scale_core.sh

echo "test scaling of core components in cluster for version v6.2.0 wo/ TLS, via easy ssh"
scale_core v6.2.0 false false
