#!/bin/bash
set -e
ulimit -n 1000000

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR=/home/tidb/deploy
cd "${DEPLOY_DIR}" || exit 1

LANG=en_US.UTF-8 \
exec opt/grafana/bin/grafana-server \
        --homepath="/home/tidb/deploy/opt/grafana" \
        --config="/home/tidb/deploy/opt/grafana/conf/grafana.ini"
