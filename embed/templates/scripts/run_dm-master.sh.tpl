#!/bin/bash
set -e

# WARNING: This file was auto-generated. Do not edit!
#          All your edit might be overwritten!
DEPLOY_DIR={{.DeployDir}}
cd "${DEPLOY_DIR}" || exit 1

{{- if .NumaNode}}
exec numactl --cpunodebind={{.NumaNode}} --membind={{.NumaNode}} bin/dm-master/dm-master \
{{- else}}
exec bin/dm-master/dm-master \
{{- end}}
{{- if .V1SourcePath}}
    --v1-sources-path="{{.V1SourcePath}}" \
{{- end}}
    --name="{{.Name}}" \
    --master-addr="{{.MasterAddr}}" \
    --advertise-addr="{{.AdvertiseAddr}}" \
    --peer-urls="{{.PeerURL}}" \
    --advertise-peer-urls="{{.AdvertisePeerURL}}" \
    --log-file="{{.LogDir}}/dm-master.log" \
    --data-dir="{{.DataDir}}" \
    --initial-cluster="{{.InitialCluster}}" \
    --config=conf/dm-master.toml >> "{{.LogDir}}/dm-master_stdout.log" 2>> "{{.LogDir}}/dm-master_stderr.log"
