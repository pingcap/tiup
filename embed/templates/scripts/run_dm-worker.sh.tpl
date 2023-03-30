#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}

cd "${DEPLOY_DIR}" || exit 1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/dm-worker/dm-worker \
{{- else}}
exec bin/dm-worker/dm-worker \
{{- end}}
    --name="{{.Name}}" \
    --worker-addr="{{.WorkerAddr}}" \
    --advertise-addr="{{.AdvertiseAddr}}" \
    --log-file="{{.LogDir}}/dm-worker.log" \
    --join="{{.Join}}" \
    --config=conf/dm-worker.toml >> "{{.LogDir}}/dm-worker_stdout.log" 2>> "{{.LogDir}}/dm-worker_stderr.log"
