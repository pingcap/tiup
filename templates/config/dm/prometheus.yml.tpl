---
global:
  scrape_interval:     15s # By default, scrape targets every 15 seconds.
  evaluation_interval: 15s # By default, scrape targets every 15 seconds.
  # scrape_timeout is set to the global default (10s).
  external_labels:
    cluster: '{{.ClusterName}}'
    monitor: "prometheus"

# Load and evaluate rules in this file every 'evaluation_interval' seconds.
rule_files:
  - 'dm_worker.rules.yml'
  - 'dm_master.rules.yml'

{{- if .AlertmanagerAddrs}}
alerting:
  alertmanagers:
  - static_configs:
    - targets:
{{- range .AlertmanagerAddrs}}
    - '{{.}}'
{{- end}}
{{- end}}

scrape_configs:
{{- if .MasterAddrs}}
  - job_name: "dm_master"
    honor_labels: true # don't overwrite job & instance labels
    static_configs:
    - targets:
    {{- range .MasterAddrs}}
       - '{{.}}'
    {{- end}}
{{- end}}

{{- if .WorkerAddrs}}
  - job_name: "dm_worker"
    honor_labels: true # don't overwrite job & instance labels
    static_configs:
    - targets:
    {{- range .WorkerAddrs}}
       - '{{.}}'
    {{- end}}
{{- end}}
