#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}

cd "${DEPLOY_DIR}" || exit 1

{{- define "PDList"}}
  {{- range $idx, $pd := .}}
    {{- if eq $idx 0}}
      {{- $pd.Scheme}}://{{$pd.IP}}:{{$pd.ClientPort}}
    {{- else -}}
      ,{{- $pd.Scheme}}://{{$pd.IP}}:{{$pd.ClientPort}}
    {{- end}}
  {{- end}}
{{- end}}

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/pump \
{{- else}}
exec bin/pump \
{{- end}}
{{- if .NodeID}}
    --node-id="{{.NodeID}}" \
{{- end}}
    --addr="0.0.0.0:{{.Port}}" \
    --advertise-addr="{{.Host}}:{{.Port}}" \
    --pd-urls="{{template "PDList" .Endpoints}}" \
    --data-dir="{{.DataDir}}" \
    --log-file="{{.LogDir}}/pump.log" \
    --config=conf/pump.toml 2>> "{{.LogDir}}/pump_stderr.log"
