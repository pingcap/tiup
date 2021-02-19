import { useLocalStorageState } from 'ahooks'
import { useMemo } from 'react'

import { MachineMap, Machine } from '_types'

export function useMachines() {
  const [machineObjs, setMachineObjs] = useLocalStorageState<MachineMap>(
    'machines',
    {}
  )

  const machines = useMemo(() => {
    let _machines: MachineMap = {}
    Object.keys(machineObjs).forEach((k) => {
      _machines[k] = Machine.deSerial(machineObjs[k])
    })
    return _machines
  }, [machineObjs])

  return { machines, setMachineObjs }
}
