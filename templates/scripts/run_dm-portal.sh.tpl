#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1


{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/dm-portal \
{{- else}}
exec bin/dm-portal \
{{- end}}
    --port="{{.Port}}" \
    --task-file-path="{{.DataDir}}" \
    --timeout="{{.Timeout}}" >> "{{.LogDir}}/dm-portal_stdout.log" 2>> "{{.LogDir}}/dm-portal_stderr.log"
