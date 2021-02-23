#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
cd "{{.DeployDir}}" || exit 1

export RUST_BACKTRACE=1

export TZ=${TZ:-/etc/localtime}
export LD_LIBRARY_PATH={{.DeployDir}}/bin/tiflash:$LD_LIBRARY_PATH

echo -n 'sync ... '
stat=$(time sync)
echo ok
echo $stat

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/tiflash/tiflash server \
{{- else}}
exec bin/tiflash/tiflash server \
{{- end}}
    --config-file conf/tiflash.toml 2>> "{{.LogDir}}/tiflash_stderr.log"
