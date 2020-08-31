import React, { useMemo } from 'react'
import yaml from 'yaml'

import { IMachine } from '../Machines/MachineForm'
import {
  BaseComp,
  DEF_DEPLOY_DIR_PREFIX,
  DEF_DATA_DIR_PREFIX,
  COMP_TYPES_ARR,
} from '../../types/comps'

interface ITopoPreviewProps {
  forScaleOut: boolean
  machines: { [key: string]: IMachine }
  components: { [key: string]: BaseComp }
}

// TODO: split into 2 methods: genDeployTopo, genScaleOutTopo
export function genTopo({
  machines,
  components,
  forScaleOut,
}: ITopoPreviewProps) {
  const componentsArr = Object.values(components)

  let topo = {} as any

  if (!forScaleOut) {
    topo = {
      global: {
        user: 'tidb',
        deploy_dir: DEF_DEPLOY_DIR_PREFIX,
        data_dir: DEF_DATA_DIR_PREFIX,
      },
      server_configs: {
        // tidb: {
        //   'performance.txn-total-size-limit': 1048576000,
        // },
        pd: {
          'replication.enable-placement-rules': true,
        },
      },
    }
  }

  for (const compType of COMP_TYPES_ARR) {
    const comps = componentsArr.filter(
      (comp) => comp.type === compType && comp.for_scale_out === forScaleOut
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
      // username / password / privateKey / deploy_dir / data_dir

      for (const key of Object.keys(comp)) {
        if (key.indexOf('port') !== -1 && comp[key] !== undefined) {
          m[key] = comp[key]
        }
        // TODO: handle deploy dir and data dir
      }

      // location labels
      if (compType === 'TiKV' && (targetMachine.dc || targetMachine.rack)) {
        m.config = {
          'server.labels': { dc: targetMachine.dc, rack: targetMachine.rack },
        }
      }

      topo[topoKey].push(m)
      topo[topoKey].sort((a: any, b: any) => (a.host > b.host ? 1 : -1))
    }
  }
  return topo
}

export default function TopoPreview({
  forScaleOut,
  machines,
  components,
}: ITopoPreviewProps) {
  const topo = useMemo(() => genTopo({ machines, components, forScaleOut }), [
    machines,
    components,
    forScaleOut,
  ])

  return (
    <div
      style={{
        maxHeight: 400,
        padding: 12,
        border: '1px solid #ccc',
        overflowY: 'auto',
      }}
    >
      {/* TODO: syntax highlight */}
      <pre>{yaml.stringify(topo)}</pre>
    </div>
  )
}
