import React, { useMemo } from 'react'
import yaml from 'yaml'

import {
  DEF_DEPLOY_DIR_PREFIX,
  DEF_DATA_DIR_PREFIX,
  COMP_TYPES_ARR,
  CompMap,
  MachineMap,
} from '_types'

interface ITopoPreviewProps {
  machines: MachineMap
  components: CompMap
  forScaleOut: boolean
}

export function genTopo(
  machines: MachineMap,
  components: CompMap,
  forScaleOut: boolean
) {
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
    )

    if (comps.length === 0) {
      continue
    }

    let topoKey = `${comps[0].name()}_servers`
    topo[topoKey] = []

    for (const comp of comps) {
      const targetMachine = machines[comp.machineID]
      let m = {} as any
      m.host = targetMachine.host
      if (targetMachine.ssh_port !== undefined) {
        m.ssh_port = targetMachine.ssh_port
      }
      // TODO:
      // username / password / privateKey

      for (const key of Object.keys(comp)) {
        if (key.indexOf('port') !== -1 && (comp as any)[key] !== undefined) {
          m[key] = (comp as any)[key]
        }
        if (key === 'deploy_dir_prefix' && comp[key] !== undefined) {
          m['deploy_dir'] = comp.deployPathFull()
        }
        if (key === 'data_dir_prefix' && comp[key] !== undefined) {
          m['data_dir'] = comp.dataPathFull()
        }
      }

      // location labels
      if (compType === 'TiKV' && (targetMachine.dc || targetMachine.rack)) {
        m.config = {
          'server.labels': { dc: targetMachine.dc, rack: targetMachine.rack },
        }
      }

      topo[topoKey].push(m)
      topo[topoKey].sort((a: any, b: any) => {
        if (a.host > b.host) {
          return 1
        }
        if (a.host < b.host) {
          return -1
        }
        const aPort = a.port || a.tcp_port || a.client_port || a.web_port || 0
        const bPort = b.port || b.tcp_port || b.client_port || b.web_port || 0
        return aPort - bPort
      })
    }
  }
  return topo
}

export default function TopoPreview({
  machines,
  components,
  forScaleOut,
}: ITopoPreviewProps) {
  const topo = useMemo(() => genTopo(machines, components, forScaleOut), [
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
