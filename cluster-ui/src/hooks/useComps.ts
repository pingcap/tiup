import { useLocalStorageState } from 'ahooks'
import { useMemo } from 'react'

import { BaseComp, CompMap } from '_types'

export function useComps() {
  const [compObjs, setCompObjs] = useLocalStorageState<CompMap>(
    'components',
    {}
  )

  const comps = useMemo(() => {
    let _comps: CompMap = {}
    Object.keys(compObjs).forEach((k) => {
      _comps[k] = BaseComp.deSerial(compObjs[k])
    })
    return _comps
  }, [compObjs])

  return { comps, setCompObjs }
}
