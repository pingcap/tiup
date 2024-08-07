#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}

cd "${DEPLOY_DIR}" || exit 1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} env GODEBUG=madvdontneed=1 bin/pd-server services scheduling\
{{- else}}
exec env GODEBUG=madvdontneed=1 bin/pd-server services scheduling \
{{- end}}
{{- if .Name}}
    --name="{{.Name}}" \
{{- end}}
    --backend-endpoints="{{.BackendEndpoints}}" \
    --listen-addr="{{.ListenURL}}" \
    --advertise-listen-addr="{{.AdvertiseListenURL}}" \
    --config=conf/scheduling.toml \
    --log-file="{{.LogDir}}/scheduling.log" 2>> "{{.LogDir}}/scheduling_stderr.log"
