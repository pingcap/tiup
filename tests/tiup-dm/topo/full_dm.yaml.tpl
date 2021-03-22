server_configs:
  master:
    rpc-timeout: "30s" # you many need update test_upgrade.sh if change this cause it's used to test edit-config
    rpc-rate-limit: 10.0
    rpc-rate-burst: 40

master_servers:
  - host: __IPPREFIX__.101
    config:
      rpc-rate-limit: 10.0
      rpc-rate-burst: 40
  - host: __IPPREFIX__.101
    port: 8361
    peer_port: 8292
  - host: __IPPREFIX__.102
  - host: __IPPREFIX__.103
    data_dir: /home/tidb/my_master_data
  - host: __IPPREFIX__.104

worker_servers:
  - host: __IPPREFIX__.101
    port: 8262
  - host: __IPPREFIX__.101
    port: 8263
  - host: __IPPREFIX__.102
  - host: __IPPREFIX__.103
  - host: __IPPREFIX__.104

monitoring_servers:
  - host: __IPPREFIX__.101
    rule_dir: /tmp/local/prometheus
grafana_servers:
  - host: __IPPREFIX__.101
    dashboard_dir: /tmp/local/grafana
alertmanager_servers:
  - host: __IPPREFIX__.101
    config_file: /tmp/local/alertmanager/alertmanager.yml

monitored:
  node_exporter_port: 39100
  blackbox_exporter_port: 39115
