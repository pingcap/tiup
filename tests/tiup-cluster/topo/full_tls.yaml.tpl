global:
  user: tidb
  group: pingcap
  enable_tls: true

server_configs:
  tidb:
    binlog.enable: true
    binlog.ignore-error: false
  tikv:
    storage.reserve-space: 1K
  pump:
    storage.stop-write-at-available-space: 1 mib

tidb_servers:
  - host: __IPPREFIX__.101
  - host: __IPPREFIX__.102

pd_servers:
  - host: __IPPREFIX__.103
  - host: __IPPREFIX__.104
  - host: __IPPREFIX__.105

# Note if only 3 instance, when scale-in one of it.
# It may not be tombstone.
tikv_servers:
  - host: __IPPREFIX__.101
  - host: __IPPREFIX__.103
    data_dir: "/home/tidb/my_kv_data"
  - host: __IPPREFIX__.104
  - host: __IPPREFIX__.105

pump_servers:
  - host: __IPPREFIX__.103
  - host: __IPPREFIX__.104
  - host: __IPPREFIX__.105

drainer_servers:
  - host: __IPPREFIX__.101
    data_dir: /home/tidb/data/drainer-8249/data
    commit_ts: -1
    config:
      syncer.db-type: "file"

cdc_servers:
  - host: __IPPREFIX__.103
  - host: __IPPREFIX__.104
  - host: __IPPREFIX__.105

monitoring_servers:
  - host: __IPPREFIX__.101
    rule_dir: /tmp/local/prometheus
grafana_servers:
  - host: __IPPREFIX__.101
    dashboard_dir: /tmp/local/grafana
alertmanager_servers:
  - host: __IPPREFIX__.101
    config_file: /tmp/local/alertmanager/alertmanager.yml
