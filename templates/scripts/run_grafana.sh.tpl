#!/bin/bash
set -e
ulimit -n 1000000

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

LANG=en_US.UTF-8 \
{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} grafana-6.1.6/bin/grafana-server \
{{- else}}
exec grafana-6.1.6/bin/grafana-server \
{{- end}}
    --homepath="{{.DeployDir}}/grafana-6.1.6" \
    --config="{{.DeployDir}}/grafana-6.1.6/conf/grafana.ini"