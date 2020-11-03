import { Button, Space } from 'antd'
import React, { useEffect, useState } from 'react'
import yaml from 'yaml'

import { COMP_TYPES_ARR, CompMap, MachineMap, GlobalDir } from '_types'

interface ITopoPreviewProps {
  globalDir: GlobalDir
  machines: MachineMap
  components: CompMap
  forScaleOut: boolean

  onTopoYamlChange: (yaml: string) => void
}

export function genTopo(
  globalDir: GlobalDir,
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
        deploy_dir: globalDir.deployPathPrefix(),
        data_dir: globalDir.dataPathPrefix(),
      },
      server_configs: {
        // tidb: {
        //   'performance.txn-total-size-limit': 1048576000,
        // },
        pd: {
          'replication.enable-placement-rules': true,
          'replication.location-labels': ['zone', 'dc', 'host'],
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
          m['deploy_dir'] = comp.deployPathFull(globalDir)
        }
        if (key === 'data_dir_prefix' && comp[key] !== undefined) {
          m['data_dir'] = comp.dataPathFull(globalDir)
        }
        if (key === 'numa_node' && (comp as any)[key] !== undefined) {
          m['numa_node'] = (comp as any)[key]
        }
      }

      // location labels
      if (compType === 'TiKV') {
        m.config = {
          'server.labels': {
            zone: targetMachine.zone || 'zone1',
            dc: targetMachine.dc || 'dc1',
            host: targetMachine.name || targetMachine.id,
          },
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
  globalDir,
  machines,
  components,
  forScaleOut,
  onTopoYamlChange,
}: ITopoPreviewProps) {
  const [topoYaml, setTopoYaml] = useState(() =>
    yaml.stringify(genTopo(globalDir, machines, components, forScaleOut))
  )
  const [editable, setEditable] = useState(false)

  function handleTopoYamlChange(event: any) {
    const val = event.target.value
    setTopoYaml(val)
    onTopoYamlChange(val)
  }

  useEffect(() => {
    onTopoYamlChange(topoYaml)
    // eslint-disable-next-line
  }, [])

  return (
    <div>
      <Space style={{ marginBottom: 8 }}>
        <Button onClick={() => setEditable(true)}>手动微调</Button>
      </Space>
      <pre>
        <textarea
          disabled={!editable}
          value={topoYaml}
          onChange={handleTopoYamlChange}
          style={{ width: '100%', height: 400, padding: 8 }}
        />
      </pre>
    </div>
  )
}
