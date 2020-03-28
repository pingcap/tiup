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
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/pd-server \
{{- else}}
exec bin/pd-server \
{{- end}}
    --name="{{.Name}}" \
    --client-urls="{{.Scheme}}://{{.IP}}:{{.ClientPort}}" \
    --advertise-client-urls="{{.Scheme}}://{{.IP}}:{{.ClientPort}}" \
    --peer-urls="{{.Scheme}}://{{.IP}}:{{.PeerPort}}" \
    --advertise-peer-urls="{{.Scheme}}://{{.IP}}:{{.PeerPort}}" \
    --data-dir="{{.DataDir}}" \
    --join="{{template "PDList" .Endpoints}}" \
    --log-file="log/pd.log" 2>> "log/pd_stderr.log"
  
