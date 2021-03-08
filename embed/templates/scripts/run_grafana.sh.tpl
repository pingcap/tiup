#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

# try to compatible with tidb-lightning's dashboard,
# the tidb-lightning dashboard's uid is not set,
# after the cluster renamed, the old dashboard will not be replaced with the newly one.
# and Grafana use the unique uid as indepent dashaboard.
# this script was stolen from https://stackoverflow.com/a/13844351
if [[ -f dashboards/lightning.json ]]; then
    grep -qxF uid dashboards/lightning.json || sed -i '$s/}/,\n"uid":"t5PjsKUGz"}/' dashboards/lightning.json
fi

LANG=en_US.UTF-8 \
{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/bin/grafana-server \
{{- else}}
exec bin/bin/grafana-server \
{{- end}}
    --homepath="{{.DeployDir}}/bin" \
    --config="{{.DeployDir}}/conf/grafana.ini"
