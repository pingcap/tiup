apiVersion: 1
datasources:
  - name: {{.ClusterName}}
    type: prometheus
    access: proxy
    url: {{.URL}}
    withCredentials: false
    isDefault: false
    tlsAuth: false
    tlsAuthWithCACert: false
    version: 1
    editable: true