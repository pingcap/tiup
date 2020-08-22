import request from './request'

const API_URL = 'http://127.0.0.1:8080/api'

function fullUrl(path: string): string {
  return `${API_URL}/${path}`
}

////////////////////

export function deployCluster(deployment: any) {
  return request(fullUrl('deploy'), 'POST', deployment)
}

export function getDeploymentStatus() {
  return request(fullUrl('deploy_status'))
}

export function getClusterList() {
  return request(fullUrl('clusters'))
}

export function deleteCluster(clusterName: string) {
  return request(fullUrl(`clusters/${clusterName}`), 'DELETE')
}
