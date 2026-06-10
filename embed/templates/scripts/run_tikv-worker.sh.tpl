#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
cd "{{.DeployDir}}" || exit 1

{{- if and .NumaNode .NumaCores}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} -C {{.NumaCores}} bin/tikv-worker \
{{- else if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/tikv-worker \
{{- else}}
exec bin/tikv-worker \
{{- end}}
    --addr "{{.Addr}}" \
    --pd-endpoints "{{.PD}}" \
    --config conf/tikv-worker.toml \
    --log-file "{{.LogDir}}/tikv_worker.log" 2>> "{{.LogDir}}/tikv_worker_stderr.log"

