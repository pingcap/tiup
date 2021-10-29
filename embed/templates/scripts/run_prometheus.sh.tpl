#!/bin/bash
set -e

DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!

if [ -e "bin/ng-monitoring-server" ]; then
echo "#!/bin/bash

# WARNING: This file was auto-generated to restart ng-monitoring when fail. 
#          Do not edit! All your edit might be overwritten!

while true
do
{{- if .NumaNode}}
    numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/ng-monitoring-server \
{{- else}}
    bin/ng-monitoring-server \
{{- end}}
        --config {{.DeployDir}}/conf/ngmonitoring.toml \
        >/dev/null 2>&1
done" > bin/ng-wrapper.sh
fi

/bin/bash bin/ng-wrapper.sh &

exec > >(tee -i -a "{{.LogDir}}/prometheus.log")
exec 2>&1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/prometheus/prometheus \
{{- else}}
exec bin/prometheus/prometheus \
{{- end}}
    --config.file="{{.DeployDir}}/conf/prometheus.yml" \
    --web.listen-address=":{{.Port}}" \
    --web.external-url="http://{{.IP}}:{{.Port}}/" \
    --web.enable-admin-api \
    --log.level="info" \
    --storage.tsdb.path="{{.DataDir}}" \
    --storage.tsdb.retention="{{.Retention}}"
