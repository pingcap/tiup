#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

exec > >(tee -i -a "{{.LogDir}}/node_exporter.log")
exec 2>&1

EXPORTER_BIN=bin/node_exporter/node_exporter
if [ ! -f $EXPORTER_BIN ]; then
  EXPORTER_BIN=bin/node_exporter
fi

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} $EXPORTER_BIN \
{{- else}}
exec $EXPORTER_BIN \
{{- end}}
    --web.listen-address=":{{.Port}}" \
    --collector.tcpstat \
    --collector.systemd \
    --collector.mountstats \
    --collector.meminfo_numa \
    --collector.interrupts \
    --collector.buddyinfo \
    --collector.vmstat.fields="^.*" \
    --log.level="info"
