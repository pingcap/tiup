#!/bin/bash

set -eu

source script/scale_core.sh

echo "test scaling of core components in cluster for version v6.0.0 w/ TLS, via easy ssh"
scale_core v6.0.0 true false
