#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}

cd "${DEPLOY_DIR}" || exit 1

{{- if and .NumaNode .NumaCores}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} -C {{.NumaCores}} env GODEBUG=madvdontneed=1 bin/tici-server \
{{- else if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} env GODEBUG=madvdontneed=1 bin/tici-server \
{{- else}}
exec env GODEBUG=madvdontneed=1 bin/tici-server \
{{- end}}
    meta \
    -P {{.Port}} \
    --status-port="{{.StatusPort}}" \
    --host="{{.ListenHost}}" \
    --advertise-host="{{.AdvertiseHost}}" \
    --pd-addr="{{.PD}}" \
    --config=conf/tici-meta.toml \
    1>> "{{.LogDir}}/tici-meta.log" \
    2>> "{{.LogDir}}/tici-meta_stderr.log"
