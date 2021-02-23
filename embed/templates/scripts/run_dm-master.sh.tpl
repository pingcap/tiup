#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

{{- define "MasterList"}}
  {{- range $idx, $master := .}}
    {{- if eq $idx 0}}
      {{- $master.Name}}={{$master.Scheme}}://{{$master.IP}}:{{$master.PeerPort}}
    {{- else -}}
      ,{{- $master.Name}}={{$master.Scheme}}://{{$master.IP}}:{{$master.PeerPort}}
    {{- end}}
  {{- end}}
{{- end}}

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/dm-master/dm-master \
{{- else}}
exec bin/dm-master/dm-master \
{{- end}}
{{- if .V1SourcePath}}
    --v1-sources-path="{{.V1SourcePath}}" \
{{- end}}
    --name="{{.Name}}" \
    --master-addr="0.0.0.0:{{.Port}}" \
    --advertise-addr="{{.IP}}:{{.Port}}" \
    --peer-urls="{{.IP}}:{{.PeerPort}}" \
    --advertise-peer-urls="{{.IP}}:{{.PeerPort}}" \
    --log-file="{{.LogDir}}/dm-master.log" \
    --data-dir="{{.DataDir}}" \
    --initial-cluster="{{template "MasterList" .Endpoints}}" \
    --config=conf/dm-master.toml >> "{{.LogDir}}/dm-master_stdout.log" 2>> "{{.LogDir}}/dm-master_stderr.log"
