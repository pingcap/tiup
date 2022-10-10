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

{{- if .RequiredCPUFlags }}
if [ -f "/proc/cpuinfo" ]; then
    IFS_OLD=$IFS
    IFS=' '
    line="{{.RequiredCPUFlags}}"
    for flag in $(echo $line); do
        grep -q ${flag} /proc/cpuinfo
        if [ $? -ne 0 ]; then
            err_msg="Fail to check CPU flags: ${flag} not supported. Require ${line}"
            echo ${err_msg}
            echo ${err_msg} >>"{{.LogDir}}/tiflash_stderr.log"
            exit -1
        fi

    done
    IFS=$IFS_OLD
fi
{{- end}}

{{- if and .NumaNode .NumaCores}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} -C {{.NumaCores}} bin/tiflash/tiflash server \
{{- else if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/tiflash/tiflash server \
{{- else}}
exec bin/tiflash/tiflash server \
{{- end}}
    --config-file conf/tiflash.toml 2>> "{{.LogDir}}/tiflash_stderr.log"
