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
      ,{{$master.IP}}:{{$master.Port}}
    {{- end}}
  {{- end}}
{{- end}}

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/dm-worker/dm-worker \
{{- else}}
exec bin/dm-worker/dm-worker \
{{- end}}
    --name="{{.Name}}" \
    --worker-addr="0.0.0.0:{{.Port}}" \
    --advertise-addr="{{.IP}}:{{.Port}}" \
    --log-file="{{.LogDir}}/dm-worker.log" \
    --join="{{template "MasterList" .Endpoints}}" \
    --config=conf/dm-worker.toml >> "{{.LogDir}}/dm-worker_stdout.log" 2>> "{{.LogDir}}/dm-worker_stderr.log"
