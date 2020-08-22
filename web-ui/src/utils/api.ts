import request from './request'

const API_URL = 'http://127.0.0.1:8080/api'

function fullUrl(path: string): string {
  return `${API_URL}/${path}`
}

////////////////////

export function getClusterList() {
  return request(fullUrl('clusters'))
}
