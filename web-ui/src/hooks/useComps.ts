import { useLocalStorageState } from 'ahooks'
import { BaseComp, CompMap } from '../types/comps'
import { useMemo } from 'react'

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
