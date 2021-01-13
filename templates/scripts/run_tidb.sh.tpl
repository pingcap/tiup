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
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} env GODEBUG=madvdontneed=1 bin/tidb-server \
{{- else}}
exec env GODEBUG=madvdontneed=1 bin/{{.Brand}}-server \
{{- end}}
    -P {{.Port}} \
    --status="{{.StatusPort}}" \
    --host="{{.ListenHost}}" \
    --advertise-address="{{.IP}}" \
    --store="tikv" \
    --path="{{template "PDList" .Endpoints}}" \
    --log-slow-query="log/{{.Brand}}_slow_query.log" \
    --config=conf/{{.Brand}}.toml \
    --log-file="{{.LogDir}}/{{.Brand}}.log" 2>> "{{.LogDir}}/{{.Brand}}_stderr.log"
