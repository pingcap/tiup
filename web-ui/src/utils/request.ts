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
    return response.json().then((resData: any) => {
      const errMsg = resData.msg || response.statusText
      const error: ResError = new Error(errMsg)
      error.response = response
      throw error
    })
  }
}
