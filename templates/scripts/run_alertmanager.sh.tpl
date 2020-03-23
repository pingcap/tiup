#!/bin/bash
set -e
ulimit -n 1000000

DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
exec > >(tee -i -a "log/alertmanager.log")
exec 2>&1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/alertmanager \
{{- else}}
exec bin/alertmanager \
{{- end}}
    --config.file="conf/alertmanager.yml" \
    --storage.path="{{.DataDir}}" \
    --data.retention=120h \
    --log.level="info" \
    --web.listen-address=":{{.WebPort}}" \
    --cluster.listen-address=":{{.ClusterPort}}"