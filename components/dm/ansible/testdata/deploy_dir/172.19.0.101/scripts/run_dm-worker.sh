#!/bin/bash
set -e
ulimit -n 1000000

DEPLOY_DIR=/home/tidb/deploy
cd "${DEPLOY_DIR}" || exit 1

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!

exec bin/dm-worker \
    --worker-addr=":8262" \
    -L="info" \
    --relay-dir="/home/tidb/deploy/relay_log" \
    --config="conf/dm-worker.toml" \
    --log-file="/home/tidb/deploy/log/dm-worker.log" >> "/home/tidb/deploy/log/dm-worker-stdout.log" 2>> "/home/tidb/deploy/log/dm-worker-stderr.log"
