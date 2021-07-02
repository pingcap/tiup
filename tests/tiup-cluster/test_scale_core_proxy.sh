#!/bin/bash

set -eu

source script/scale_core.sh

echo "test scaling of core components in cluster for version v4.0.12 wo/ TLS, via easy ssh"
scale_core v4.0.12 false false true
