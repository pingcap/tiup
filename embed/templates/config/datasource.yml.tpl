apiVersion: 1
deleteDatasources:
  - name: {{.ClusterName}}
datasources:
  - name: {{.ClusterName}}
    type: prometheus
    access: proxy
    url: http://{{.IP}}:{{.Port}}
    withCredentials: false
    isDefault: false
    tlsAuth: false
    tlsAuthWithCACert: false
    version: 1
    editable: true