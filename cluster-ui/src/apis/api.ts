import request from './request'

const API_URL =
  process.env.NODE_ENV === 'production' ? '/api' : 'http://127.0.0.1:8080/api'

function fullUrl(path: string): string {
  return `${API_URL}/${path}`
}

////////////////////

export function deployCluster(deployment: any) {
  return request(fullUrl('deploy'), 'POST', deployment)
}

export function getStatus() {
  return request(fullUrl('status'))
}

export function getClusterList() {
  return request(fullUrl('clusters'))
}

export function deleteCluster(clusterName: string) {
  return request(fullUrl(`clusters/${clusterName}`), 'DELETE')
}

export function getClusterTopo(clusterName: string) {
  return request(fullUrl(`clusters/${clusterName}`))
}

export function startCluster(clusterName: string) {
  return request(fullUrl(`clusters/${clusterName}/start`), 'POST')
}

export function stopCluster(clusterName: string) {
  return request(fullUrl(`clusters/${clusterName}/stop`), 'POST')
}

export function scaleInCluster(
  clusterName: string,
  scaleInOpts: { nodes: string[]; force: boolean }
) {
  return request(
    fullUrl(`clusters/${clusterName}/scale_in`),
    'POST',
    scaleInOpts
  )
}

export function scaleOutCluster(clusterName: string, scaleOutOpts: any) {
  return request(
    fullUrl(`clusters/${clusterName}/scale_out`),
    'POST',
    scaleOutOpts
  )
}

export function checkCluster(clusterName: string) {
  return request(fullUrl(`clusters/${clusterName}/check`), 'POST')
}

export function getCheckClusterResult(clusterName: string) {
  return request(fullUrl(`clusters/${clusterName}/check_result`), 'GET')
}

export function getMirrorAddress() {
  return request(fullUrl(`mirror`))
}

export function setMirrorAddress(newAddress: string) {
  return request(fullUrl(`mirror`), 'POST', {
    mirror_address: newAddress,
  })
}

export function getTiDBVersions() {
  return request(fullUrl(`tidb_versions`))
}
