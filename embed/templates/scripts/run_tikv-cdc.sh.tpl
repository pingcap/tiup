#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/tikv-cdc server \
{{- else}}
exec bin/tikv-cdc server \
{{- end}}
    --addr "{{.Addr}}" \
    --advertise-addr "{{.AdvertiseAddr}}" \
    --pd "{{.PD}}" \
{{- if .DataDir}}
    --data-dir="{{.DataDir}}" \
{{- end}}
{{- if .TLSEnabled}}
    --ca tls/ca.crt \
    --cert tls/cdc.crt \
    --key tls/cdc.pem \
{{- end}}
{{- if .GCTTL}}
    --gc-ttl {{.GCTTL}} \
{{- end}}
{{- if .TZ}}
    --tz "{{.TZ}}" \
{{- end}}
    --config conf/tikv-cdc.toml \
    --log-file "{{.LogDir}}/tikv-cdc.log" 2>> "{{.LogDir}}/tikv-cdc_stderr.log"
