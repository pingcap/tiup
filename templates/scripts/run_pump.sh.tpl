#!/bin/bash
set -e
ulimit -n 1000000

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}

cd "${DEPLOY_DIR}" || exit 1

{{- define "PDList"}}
  {{- range $idx, $pd := .}}
    {{- if eq $idx 0}}
      {{- $pd.Scheme}}://{{$pd.IP}}:{{$pd.PeerPort}}
    {{- else -}}
      ,{{- $pd.Scheme}}://{{$pd.IP}}:{{$pd.PeerPort}}
    {{- end}}
  {{- end}}
{{- end}}

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/pump \
{{- else}}
exec bin/pump \
{{- end}}
    --node-id="{{.NodeID}}" \
    --addr="0.0.0.0:{{.Port}}" \
    --advertise-addr="{{.IP}}:{{.Port}}" \
    --pd-urls="{{template "PDList" .Endpoints}}" \
    --data-dir="{{.DataDir}}" \
    --log-file="log/pump.log" \
    --config=conf/pump.toml 2>> "log/pump_stderr.log"