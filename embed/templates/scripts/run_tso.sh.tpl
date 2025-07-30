#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}

cd "${DEPLOY_DIR}" || exit 1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} env GODEBUG=madvdontneed=1 bin/pd-server services tso\
{{- else}}
exec env GODEBUG=madvdontneed=1 bin/pd-server services tso \
{{- end}}
{{- if .Name}}
    --name="{{.Name}}" \
{{- end}}
    --backend-endpoints="{{.BackendEndpoints}}" \
    --listen-addr="{{.ListenURL}}" \
    --advertise-listen-addr="{{.AdvertiseListenURL}}" \
    --config=conf/tso.toml \
    --log-file="{{.LogDir}}/tso.log" 2>> "{{.LogDir}}/tso_stderr.log"
