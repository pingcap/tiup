# The topology template is used deploy a minimal DM cluster, which suitable
# for scenarios with only three machinescontains. The minimal cluster contains
# - 3 master nodes
# - 3 worker nodes
# You can change the hosts according your environment
---
global:
  # # The OS user who runs the tidb cluster.
  user: "{{ .GlobalUser }}"
  {{- if .GlobalGroup }}
  # group is used to specify the group name the user belong to if it's not the same as user.
  group: "{{ .GlobalGroup }}"
  {{- end }}
  {{- if .GlobalSystemdMode }}
  # # systemd_mode is used to select whether to use sudo permissions.
  systemd_mode: "{{ .GlobalSystemdMode }}"
  {{- end }}
  # # SSH port of servers in the managed cluster.
  ssh_port: {{ .GlobalSSHPort }}
  # # Storage directory for cluster deployment files, startup scripts, and configuration files.
  deploy_dir: "{{ .GlobalDeployDir }}"
  # # TiDB Cluster data storage directory
  data_dir: "{{ .GlobalDataDir }}"
  {{- if .GlobalArch }}
  # # Supported values: "amd64", "arm64" (default: "amd64")
  arch: "{{ .GlobalArch }}"
  {{- end }}

{{ if .MasterServers -}}
master_servers:
{{- range .MasterServers }}
  - host: {{ . }}
{{- end }}
{{ end }}
{{ if .WorkerServers -}}
worker_servers:
{{- range .WorkerServers }}
  - host: {{ . }}
{{- end }}
{{ end }}
{{ if .MonitoringServers -}}
monitoring_servers:
 {{- range .MonitoringServers }}
  - host: {{ . }}
{{- end }}
{{ end }}
{{ if .GrafanaServers -}}
grafana_servers:
 {{- range .GrafanaServers }}
  - host: {{ . }}
{{- end }}
{{ end }}
{{- if .AlertManagerServers }}
alertmanager_servers:
 {{- range .AlertManagerServers }}
  - host: {{ . }}
{{- end }}
{{ end }}
