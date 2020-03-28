#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}

cd "${DEPLOY_DIR}" || exit 1

{{- define "PDList"}}
  {{- range $idx, $pd := .}}
    {{- if eq $idx 0}}
      {{- $pd.IP}}:{{$pd.ClientPort}}
    {{- else -}}
      ,{{$pd.IP}}:{{$pd.ClientPort}}
    {{- end}}
  {{- end}}
{{- end}}

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/tidb-server \
{{- else}}
exec bin/tidb-server \
{{- end}}
    -P {{.Port}} \
    --status="{{.StatusPort}}" \
    --advertise-address="{{.IP}}" \
    --store="tikv" \
    --path="{{template "PDList" .Endpoints}}" \
    --log-slow-query="log/tidb_slow_query.log" \
    --log-file="log/tidb.log" 2>> "log/tidb_stderr.log"
