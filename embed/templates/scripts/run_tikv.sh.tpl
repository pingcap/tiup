#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
cd "{{.DeployDir}}" || exit 1

echo -n 'sync ... '
stat=$(time sync || sync)
echo ok
echo $stat

export MALLOC_CONF="prof:true,prof_active:false"

{{- if and .NumaNode .NumaCores}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} -C {{.NumaCores}} bin/tikv-server \
{{- else if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/tikv-server \
{{- else}}
exec bin/tikv-server \
{{- end}}
    --addr "{{.Addr}}" \
    --advertise-addr "{{.AdvertiseAddr}}" \
    --status-addr "{{.StatusAddr}}" \
{{- if .SupportAdvertiseStatusAddr}}
    --advertise-status-addr "{{.AdvertiseStatusAddr}}" \
{{- end}}
    --pd "{{.PD}}" \
    --data-dir "{{.DataDir}}" \
    --config conf/tikv.toml \
    --log-file "{{.LogDir}}/tikv.log" 2>> "{{.LogDir}}/tikv_stderr.log"
