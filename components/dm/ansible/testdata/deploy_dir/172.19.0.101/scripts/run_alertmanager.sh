#!/bin/bash
set -e
ulimit -n 1000000

DEPLOY_DIR=/home/tidb/deploy
cd "${DEPLOY_DIR}" || exit 1

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
exec > >(tee -i -a "/home/tidb/deploy/log/alertmanager.log")
exec 2>&1

exec bin/alertmanager \
    --config.file="conf/alertmanager.yml" \
    --storage.path="/home/tidb/deploy/data.alertmanager" \
    --data.retention=120h \
    --log.level="info" \
    --web.listen-address=":9093"
