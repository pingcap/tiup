# For more information about the format of the tiup cluster topology file, consult
# https://docs.pingcap.com/tidb/stable/production-deployment-using-tiup#step-3-initialize-cluster-topology-file

# # Global variables are applied to all deployments and used as the default value of
# # the deployments if a specific deployment value is missing.
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

{{ if .TiDBServers -}}
pd_servers:
{{- range .PDServers }}
  - host: {{ . }}
{{- end }}
{{ end }}
{{ if .TiDBServers -}}
tidb_servers:
{{- range .TiDBServers }}
  - host: {{ . }}
{{- end }}
{{ end }}
{{ if .TiKVServers -}}
tikv_servers:
{{- range .TiKVServers }}
  - host: {{ . }}
{{- end }}
{{ end }}
{{- if .TiFlashServers }}
tiflash_servers:
 {{- range .TiFlashServers }}
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
