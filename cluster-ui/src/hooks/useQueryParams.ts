import { useMemo } from 'react'
import { useLocation } from 'react-router'

export function useQueryParams() {
  const { search } = useLocation()

  const params = useMemo(() => {
    const searchParams = new URLSearchParams(search)
    let _params: { [k: string]: any } = {}
    for (const [k, v] of searchParams) {
      _params[k] = v
    }
    return _params
  }, [search])

  return params
}
