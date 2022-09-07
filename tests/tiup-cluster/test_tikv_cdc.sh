#!/bin/bash

set -eu

source script/cmd_subtest.sh
source script/tikv_cdc.sh

###############################################
echo "Test common cases for TiKV-CDC"
cmd_subtest --version 6.2.0 --topo tikv_cdc

###############################################
# TODO: test tls cases after TiKV-CDC support TLS.

###############################################
echo "Test specified cases for TiKV-CDC"
tikv_cdc_test --version 6.2.0 --topo tikv_cdc --tikv-cdc-patch 1.0.0

###############################################
# NOTE: As TiKV-CDC always use the latest version, the upgrade does NOT cover the upgrade of TiKV-CDC itself.
#   And there is only one version compatible to TiKV-CDC (v6.2.0) now, which causes difficult to test.
# TODO: test upgrade of TiKV-CDC

###############################################
echo "Test scale in/out for TiKV-CDC" 
tikv_cdc_scale_test --version "6.2.0" --topo tikv_cdc
