#!/bin/bash
set -e
ulimit -n 1000000

DEPLOY_DIR=/home/tidb/deploy
cd "${DEPLOY_DIR}" || exit 1

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!

exec bin/dm-master \
    --master-addr=":8261" \
    -L="info" \
    --config="conf/dm-master.toml" \
    --log-file="/home/tidb/deploy/log/dm-master.log" >> "/home/tidb/deploy/log/dm-master-stdout.log" 2>> "/home/tidb/deploy/log/dm-master-stderr.log"
