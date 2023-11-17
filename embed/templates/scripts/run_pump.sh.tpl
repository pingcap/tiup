#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}

cd "${DEPLOY_DIR}" || exit 1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/pump \
{{- else}}
exec bin/pump \
{{- end}}
{{- if .NodeID}}
    --node-id="{{.NodeID}}" \
{{- end}}
    --addr="{{.Addr}}" \
    --advertise-addr="{{.AdvertiseAddr}}" \
    --pd-urls="{{.PD}}" \
    --data-dir="{{.DataDir}}" \
    --log-file="{{.LogDir}}/pump.log" \
    --config=conf/pump.toml 2>> "{{.LogDir}}/pump_stderr.log"
