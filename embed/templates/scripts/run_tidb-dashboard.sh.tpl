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
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/tidb-dashboard \
{{- else}}
exec bin/tidb-dashboard \
{{- end}}
    --feature-version="{{.TidbVersion}}" \
    --host="{{.IP}}" \
    --port="{{.Port}}" \
    --pd="{{template "PDList" .Endpoints}}" \
    --data-dir="{{.DataDir}}" \
{{- if .TLSEnabled}}
    --tidb-ca tls/ca.crt \
    --tidb-cert tls/tidb-dashboard.crt \
    --tidb-key tls/tidb-dasboard.pem \
{{- end}}
    1>> "{{.LogDir}}/tidb_dashboard.log" \
    2>> "{{.LogDir}}/tidb_dashboard_stderr.log"
