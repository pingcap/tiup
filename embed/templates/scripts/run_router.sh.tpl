#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}

cd "${DEPLOY_DIR}" || exit 1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} env GODEBUG=madvdontneed=1 bin/pd-server services router\
{{- else}}
exec env GODEBUG=madvdontneed=1 bin/pd-server services router \
{{- end}}
{{- if .Name}}
    --name="{{.Name}}" \
{{- end}}
    --backend-endpoints="{{.BackendEndpoints}}" \
    --listen-addr="{{.ListenURL}}" \
    --advertise-listen-addr="{{.AdvertiseListenURL}}" \
    --config=conf/router.toml \
    --log-file="{{.LogDir}}/router.log" 2>> "{{.LogDir}}/router_stderr.log"
