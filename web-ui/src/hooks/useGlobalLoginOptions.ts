import { useLocalStorageState } from 'ahooks'

import { IGlobalLoginOptions } from '_types'

export function useGlobalLoginOptions() {
  const [globalLoginOptions, setGlobalLoginOptions] = useLocalStorageState<
    IGlobalLoginOptions
  >('global_login_options', {})

  return {
    globalLoginOptions,
    setGlobalLoginOptions,
  }
}
