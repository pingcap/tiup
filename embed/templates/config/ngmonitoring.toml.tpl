# NG Monitoring Server Configuration.

# Server address.
address = "{{.Address}}"
advertise-address = "{{.AdvertiseAddress}}"

[log]
# Log path
path = "{{.LogDir}}"

# Log level: INFO, WARN, ERROR
level = "INFO"

[pd]
# Addresses of PD instances within the TiDB cluster. Multiple addresses are separated by commas, e.g. "10.0.0.1:2379","10.0.0.2:2379"
endpoints = [{{.PDAddrs}}]

{{- if .TLSEnabled}}
[security]
ca-path = "{{.DeployDir}}/tls/ca.crt"
cert-path = "{{.DeployDir}}/tls/prometheus.crt"
key-path = "{{.DeployDir}}/tls/prometheus.pem"
{{- end}}

[storage]
path = "{{.DataDir}}"