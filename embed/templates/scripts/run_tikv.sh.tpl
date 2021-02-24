#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
cd "{{.DeployDir}}" || exit 1

echo -n 'sync ... '
stat=$(time sync || sync)
echo ok
echo $stat

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
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/tikv-server \
{{- else}}
exec bin/tikv-server \
{{- end}}
    --addr "{{.ListenHost}}:{{.Port}}" \
    --advertise-addr "{{.IP}}:{{.Port}}" \
    --status-addr "{{.ListenHost}}:{{.StatusPort}}" \
{{- if .SupportAdvertiseStatusAddr}}
    --advertise-status-addr "{{.IP}}:{{.StatusPort}}" \
{{- end}}
    --pd "{{template "PDList" .Endpoints}}" \
    --data-dir "{{.DataDir}}" \
    --config conf/tikv.toml \
    --log-file "{{.LogDir}}/tikv.log" 2>> "{{.LogDir}}/tikv_stderr.log"
