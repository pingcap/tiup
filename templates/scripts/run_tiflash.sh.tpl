#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}

cd "${DEPLOY_DIR}" || exit 1

export RUST_BACKTRACE=1

export LD_LIBRARY_PATH={{.DeployDir}}/bin/tiflash:$LD_LIBRARY_PATH

echo -n 'sync ... '
stat=$(time sync || sync)
echo ok
echo $stat

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}}  \
{{- else}}
exec \
{{- end}}
    bin/tiflash/tiflash server --config-file conf/tiflash.toml