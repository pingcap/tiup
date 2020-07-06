#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

{{- define "MasterList"}}
  {{- range $idx, $master := .}}
    {{- if eq $idx 0}}
      {{- $master.IP}}:{{$master.Port}}
    {{- else -}}
      ,{{- $master.IP}}:{{$master.Port}}
    {{- end}}
  {{- end}}
{{- end}}

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/bin/dm-master \
{{- else}}
exec bin/bin/dm-master \
{{- end}}
    --name="{{.Name}}" \
    --master-addr="0.0.0.0:{{.Port}}" \
    --advertise-addr="{{.IP}}:{{.Port}}" \
    --peer-urls="{{.Scheme}}://{{.IP}}:{{.PeerPort}}" \
    --advertise-peer-urls="{{.Scheme}}://{{.IP}}:{{.PeerPort}}" \
    --log-file="{{.LogDir}}/dm-master.log" \
    --data-dir="{{.DataDir}}" \
    --join="{{template "MasterList" .Endpoints}}" \
    --config=conf/dm-master.toml 2>> "{{.LogDir}}/dm-master_stderr.log"
