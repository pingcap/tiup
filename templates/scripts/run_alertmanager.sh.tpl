#!/bin/bash
set -e

DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!

exec > >(tee -i -a "{{.LogDir}}/alertmanager.log")
exec 2>&1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/alertmanager \
{{- else}}
exec bin/alertmanager/alertmanager \
{{- end}}
    --config.file="conf/alertmanager.yml" \
    --storage.path="{{.DataDir}}" \
    --data.retention=120h \
    --log.level="info" \
    --web.listen-address="{{.IP}}:{{.WebPort}}" \
{{- if .EndPoints}}
{{- range $idx, $am := .EndPoints}}
    --cluster.peer="{{$am.IP}}:{{$am.ClusterPort}}" \
{{- end}}
{{- end}}
    --cluster.listen-address="{{.IP}}:{{.ClusterPort}}"
