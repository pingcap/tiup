import { message, notification } from 'antd'

import { clearAuthToken, getAuthTokenAsBearer } from '_utils/auth'

export type ResError = Error & {
  response?: any
}

export default function request(
  url: string,
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE',
  body?: object,
  options?: RequestInit
) {
  const opts: RequestInit = {
    ...options,
    method: method || 'GET',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      Authorization: getAuthTokenAsBearer() || '',
      ...options?.headers,
    },
  }
  if (body) {
    opts.body = JSON.stringify(body)
  }
  return doFetch(url, opts)
}

function doFetch(url: string, options: RequestInit) {
  return fetch(url, options)
    .then(parseResponse)
    .then((data) => ({ data, err: undefined }))
    .catch((err) => ({ data: undefined, err }))
}

function parseResponse(response: Response) {
  if (response.status === 204) {
    return {} as any
  } else if (response.status >= 200 && response.status < 300) {
    return response.json()
  } else {
    let errMsg = response.statusText
    return response
      .json()
      .then((resData: any) => {
        errMsg = resData.msg || resData.message || response.statusText
      })
      .finally(() => {
        if (
          response.url.startsWith('http://127.0.0.1') ||
          response.url.startsWith('http://localhost') ||
          response.url.startsWith(window.location.origin)
        ) {
          if (response.status === 401) {
            message.error({ content: errMsg, key: '401' })
            clearAuthToken()

            window.location.hash = '/login'
          } else {
            notification.error({ message: errMsg })
          }
        }

        const error: ResError = new Error(errMsg)
        error.response = response
        throw error
      })
  }
}
