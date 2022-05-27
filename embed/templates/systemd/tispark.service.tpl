[Unit]
Description={{.ServiceName}} service
After=syslog.target network.target remote-fs.target nss-lookup.target

[Service]
User={{.User}}
{{- if ne .JavaHome ""}}
Environment="JAVA_HOME={{.JavaHome}}"
{{- end}}
ExecStart=/bin/bash -c '{{.DeployDir}}/sbin/start-{{.ServiceName}}.sh'
ExecStop=/bin/bash -c '{{.DeployDir}}/sbin/stop-{{.ServiceName}}.sh'
Type=forking
{{- if .Restart}}
Restart={{.Restart}}
{{else}}
Restart=always
{{- end}}
RestartSec=15s
SendSIGKILL=no

[Install]
WantedBy=multi-user.target
