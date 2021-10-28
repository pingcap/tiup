#!/bin/bash
set -e

function hup_handle {
    childpid=$(cat /proc/$$/task/$$/children)
    child=($childpid)
    kill -HUP ${child[@]:0:2}
    wait
}
trap hup_handle HUP

DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!

if [ -e "bin/ng-monitoring-server" ]; then
{{- if .NumaNode}}
numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/ng-monitoring-server \
{{- else}}
bin/ng-monitoring-server \
{{- end}}
    --addr "0.0.0.0:{{.NgPort}}" \
    --pd.endpoints {{.PdList}} \
    --log.path "{{.LogDir}}" \
    >/dev/null 2>&1 &
ng_pid=$!
fi

{{- if .NumaNode}}
numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/prometheus/prometheus \
{{- else}}
bin/prometheus/prometheus \
{{- end}}
    --config.file="{{.DeployDir}}/conf/prometheus.yml" \
    --web.listen-address=":{{.Port}}" \
    --web.external-url="http://{{.IP}}:{{.Port}}/" \
    --web.enable-admin-api \
    --log.level="info" \
    --storage.tsdb.path="{{.DataDir}}" \
    --storage.tsdb.retention="{{.Retention}}" \
    2>&1 | tee -i -a "{{.LogDir}}/prometheus.log" &
prometheus_pid=$!

#trap 'kill $ng_pid $prometheus_pid; exit' CHLD
wait