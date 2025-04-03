apiVersion: 1
datasources:
{{- range .Datasources}}
  - name: {{.Name}}
    type: {{.Type}}
    access: proxy
    url: {{.URL}}
    withCredentials: false
    isDefault: {{.IsDefault}}
    tlsAuth: false
    tlsAuthWithCACert: false
    version: 1
    editable: true
{{- end}}