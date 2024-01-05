#!/bin/bash
set -e

DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!

exec > >(tee -i -a "{{.LogDir}}/alertmanager.log")
exec 2>&1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/alertmanager/alertmanager \
{{- else}}
exec bin/alertmanager/alertmanager \
{{- end}}
    --config.file="conf/alertmanager.yml" \
    --storage.path="{{.DataDir}}" \
    --data.retention=120h \
    --log.level="info" \
    --web.listen-address="{{.WebListenAddr}}" \
    --web.external-url="{{.WebExternalURL}}" \
{{- if .ClusterPeers}}
{{- range $idx, $am := .ClusterPeers}}
    --cluster.peer="{{$am}}" \
{{- end}}
{{- end}}
    --cluster.listen-address="{{.ClusterListenAddr}}"
