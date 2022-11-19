#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}

cd "${DEPLOY_DIR}" || exit 1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/drainer \
{{- else}}
exec bin/drainer \
{{- end}}
{{- if .NodeID}}
    --node-id="{{.NodeID}}" \
{{- end}}
    --addr="{{.Addr}}" \
    --pd-urls="{{.PD}}" \
    --data-dir="{{.DataDir}}" \
    --log-file="{{.LogDir}}/drainer.log" \
    --config=conf/drainer.toml 2>> "{{.LogDir}}/drainer_stderr.log"
