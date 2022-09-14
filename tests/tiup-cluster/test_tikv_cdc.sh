#!/bin/bash

set -eu

source script/tikv_cdc.sh

# TODO: test tls after TLS is supported.
# TODO: test upgrade of TiKV-CDC (there is only one release version now)

###############################################
echo -e "\033[0;36m<<< Test specified cases for TiKV-CDC >>>\033[0m"
tikv_cdc_test --version 6.2.0 --topo tikv_cdc --tikv-cdc-patch 1.0.0

###############################################
echo -e "\033[0;36m<<< Test scale in/out for TiKV-CDC >>>\033[0m"
tikv_cdc_scale_test --version 6.2.0 --topo tikv_cdc
