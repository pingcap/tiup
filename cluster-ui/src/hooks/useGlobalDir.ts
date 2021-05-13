import { useLocalStorageState } from 'ahooks'

import { GlobalDir, IGlobalDir } from '_types'
import { useMemo } from 'react'

export function useGlobalDir() {
  const [globalDirObj, setGlobalDirObj] = useLocalStorageState<IGlobalDir>(
    'global_dir',
    {}
  )

  const globalDir = useMemo(() => {
    return GlobalDir.deSerial(globalDirObj)
  }, [globalDirObj])

  return {
    globalDir,
    setGlobalDirObj,
  }
}
