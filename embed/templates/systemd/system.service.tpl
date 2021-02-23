[Unit]
Description={{.ServiceName}} service
After=syslog.target network.target remote-fs.target nss-lookup.target

[Service]
{{- if .MemoryLimit}}
MemoryLimit={{.MemoryLimit}}
{{- end}}
{{- if .CPUQuota}}
CPUQuota={{.CPUQuota}}
{{- end}}
{{- if .IOReadBandwidthMax}}
IOReadBandwidthMax={{.IOReadBandwidthMax}}
{{- end}}
{{- if .IOWriteBandwidthMax}}
IOWriteBandwidthMax={{.IOWriteBandwidthMax}}
{{- end}}
{{- if .LimitCORE}}
LimitCORE={{.LimitCORE}}
{{- end}}
LimitNOFILE=1000000
LimitSTACK=10485760

User={{.User}}
ExecStart={{.DeployDir}}/scripts/run_{{.ServiceName}}.sh
{{- if eq .ServiceName "prometheus"}}
ExecReload=/bin/kill -HUP $MAINPID
{{end}}

{{- if .Restart}}
Restart={{.Restart}}
{{else}}
Restart=always
{{end}}
RestartSec=15s
{{- if .DisableSendSigkill}}
SendSIGKILL=no
{{- end}}

[Install]
WantedBy=multi-user.target
