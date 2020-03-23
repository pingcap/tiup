apiVersion: 1
providers:
  - name: {{.ClusterName}}
    folder: {{.ClusterName}}
    type: file
    disableDeletion: false
    editable: true
    updateIntervalSeconds: 30
    options:
      path: {{.DeployDir}}/grafana-6.1.6/dashboards/{{.ClusterName}}