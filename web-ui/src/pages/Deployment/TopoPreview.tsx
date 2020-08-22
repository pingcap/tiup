import React, { useMemo } from 'react'
import yaml from 'yaml'

import { IMachine } from '../Machines/MachineForm'
import { IComponent, COMPONENT_TYPES } from './DeploymentTable'

interface ITopoPreviewProps {
  machines: { [key: string]: IMachine }
  components: { [key: string]: IComponent }
}

export function genTopo({ machines, components }: ITopoPreviewProps) {
  const componentsArr = Object.values(components)

  let topo = {} as any
  for (const compType of COMPONENT_TYPES) {
    const comps = componentsArr.filter(
      (comp) => comp.type === compType
    ) as any[]
    if (comps.length === 0) {
      continue
    }

    let topoKey = ''
    if (compType === 'Prometheus') {
      topoKey = 'monitoring_servers'
    } else {
      topoKey = `${compType.toLowerCase()}_servers`
    }
    topo[topoKey] = []

    for (const comp of comps) {
      const targetMachine = machines[comp.machineID]
      let m = {} as any
      m.host = targetMachine.host
      if (targetMachine.ssh_port !== undefined) {
        m.ssh_port = targetMachine.ssh_port
      }
      // TODO:
      // username / password / privateKey ...

      for (const key of Object.keys(comp)) {
        if (key.indexOf('port') !== -1 && comp[key] !== undefined) {
          m[key] = comp[key]
        }
        // TODO: handle deploy dir and data dir
      }

      topo[topoKey].push(m)
      topo[topoKey].sort((a: any, b: any) => (a.host > b.host ? 1 : -1))
    }
  }
  return topo
}

export default function TopoPreview({
  machines,
  components,
}: ITopoPreviewProps) {
  const topo = useMemo(() => genTopo({ machines, components }), [
    machines,
    components,
  ])

  return (
    <div>
      {/* TODO: syntax highlight */}
      <pre>{yaml.stringify(topo)}</pre>
    </div>
  )
}
