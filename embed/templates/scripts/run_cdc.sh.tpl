#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/cdc server \
{{- else}}
exec bin/cdc server \
{{- end}}
    --addr "{{.Addr}}" \
    --advertise-addr "{{.AdvertiseAddr}}" \
    --pd "{{.PD}}" \
{{- if .DataDir}}
  {{- if .DataDirEnabled}}
    --data-dir="{{.DataDir}}" \
  {{- else}}
    --sort-dir="{{.DataDir}}/tmp/sorter" \
  {{- end}}
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
{{- if .ClusterID}}
    --cluster-id {{.ClusterID}} \
{{- end}}
{{- if .ConfigFileEnabled}}
    --config conf/cdc.toml \
{{- end}}
    --log-file "{{.LogDir}}/cdc.log" 2>> "{{.LogDir}}/cdc_stderr.log"
