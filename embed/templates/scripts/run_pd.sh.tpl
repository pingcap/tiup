#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}

cd "${DEPLOY_DIR}" || exit 1

exec \
{{- if .NumaNode}}
    numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} \
{{- end}}
    env GODEBUG=madvdontneed=1 \
{{- if .MSMode}}
    PD_SERVICE_MODE=api \
{{- end}}
    bin/pd-server \
    --name="{{.Name}}" \
    --client-urls="{{.ClientURL}}" \
    --advertise-client-urls="{{.AdvertiseClientURL}}" \
    --peer-urls="{{.PeerURL}}" \
    --advertise-peer-urls="{{.AdvertisePeerURL}}" \
    --data-dir="{{.DataDir}}" \
    --initial-cluster="{{.InitialCluster}}" \
    --config=conf/pd.toml \
    --log-file="{{.LogDir}}/pd.log" 2>> "{{.LogDir}}/pd_stderr.log"
