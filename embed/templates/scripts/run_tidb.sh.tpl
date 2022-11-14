#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}

cd "${DEPLOY_DIR}" || exit 1

{{- if and .NumaNode .NumaCores}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} -C {{.NumaCores}} env GODEBUG=madvdontneed=1 bin/tidb-server \
{{- else if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} env GODEBUG=madvdontneed=1 bin/tidb-server \
{{- else}}
exec env GODEBUG=madvdontneed=1 bin/tidb-server \
{{- end}}
    -P {{.Port}} \
    --status="{{.StatusPort}}" \
    --host="{{.ListenHost}}" \
    --advertise-address="{{.AdvertiseAddr}}" \
    --store="tikv" \
{{- if .SupportSecboot}}
    --initialize-insecure \
{{- end}}
    --path="{{.PD}}" \
    --log-slow-query="{{.LogDir}}/tidb_slow_query.log" \
    --config=conf/tidb.toml \
    --log-file="{{.LogDir}}/tidb.log" 2>> "{{.LogDir}}/tidb_stderr.log"
