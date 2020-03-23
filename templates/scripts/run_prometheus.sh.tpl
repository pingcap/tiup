#!/bin/bash
set -e
ulimit -n 1000000

DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
exec > >(tee -i -a "log/prometheus.log")
exec 2>&1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/prometheus \
{{- else}}
exec bin/prometheus \
{{- end}}
    --config.file="{{.DeployDir}}/conf/prometheus.yml" \
    --web.listen-address=":{{.Port}}" \
    --web.external-url="http://{{.IP}}:{{.Port}}/" \
    --web.enable-admin-api \
    --log.level="info" \
    --storage.tsdb.path="{{.DataDir}}/prometheus_data" \
    --storage.tsdb.retention="30d"