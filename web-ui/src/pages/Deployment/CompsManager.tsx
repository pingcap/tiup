import React, { useCallback, useState } from 'react'
import { useLocalStorageState } from 'ahooks'
import { Drawer, Button, Modal, Form, Input, Select } from 'antd'
import yaml from 'yaml'
import { useNavigate } from 'react-router-dom'

import { Root } from '_components'
import { BaseComp, CompTypes, Machine } from '_types'
import { useComps, useGlobalLoginOptions, useMachines } from '_hooks'
import { deployCluster, scaleOutCluster } from '_apis'

import DeploymentTable from './DeploymentTable'
import EditCompForm from './EditCompForm'
import TopoPreview, { genTopo } from './TopoPreview'

// TODO: fetch from API
const TIDB_VERSIONS = [
  'nightly',
  'v4.0.5',
  'v4.0.4',
  'v4.0.3',
  'v4.0.2',
  'v4.0.1',
  'v4.0.0',
  'v3.1.2',
  'v3.1.1',
  'v3.1.0',
]

export interface IDeployReq {
  cluster_name: string
  tidb_version: string
}

export interface ICompsManagerProps {
  clusterName?: string
  forScaleOut: boolean
}

export default function CompsManager({
  clusterName,
  forScaleOut,
}: ICompsManagerProps) {
  const { machines } = useMachines()
  const { comps, setCompObjs } = useComps()
  const [curComp, setCurComp] = useState<BaseComp | undefined>(undefined)

  const [previewYaml, setPreviewYaml] = useState(false)

  const [deployReq, setDeployReq] = useLocalStorageState<IDeployReq>(
    'deploy_req',
    { cluster_name: '', tidb_version: '' }
  )

  const { globalLoginOptions } = useGlobalLoginOptions()

  const [, setCurScaleOutNodes] = useLocalStorageState(
    'cur_scale_out_nodes',
    {}
  )

  const navigate = useNavigate()

  const [form] = Form.useForm()

  const handleAddComponent = useCallback(
    (machine: Machine, componentType: CompTypes, forScaleOut: boolean) => {
      let comp = BaseComp.create(componentType, machine.id, forScaleOut)
      const existedSameComps = Object.values(comps).filter(
        (comp) => comp.type === componentType && comp.machineID === machine.id
      )
      if (existedSameComps.length > 0) {
        const lastComp = existedSameComps[existedSameComps.length - 1]
        comp.copyPaths(lastComp)
        comp.increasePorts(lastComp)
      }
      setCompObjs({
        ...comps,
        [comp.id]: comp,
      })
    },
    [comps, setCompObjs]
  )

  const handleUpdateComponent = useCallback(
    (comp: BaseComp) => {
      setCompObjs({
        ...comps,
        [comp.id]: comp,
      })
      setCurComp(undefined)
    },
    [comps, setCompObjs]
  )

  const handleDeleteComponent = useCallback(
    (comp: BaseComp) => {
      const newComps = { ...comps }
      delete newComps[comp.id]
      setCompObjs(newComps)
    },
    [comps, setCompObjs]
  )

  const handleDeleteComponents = useCallback(
    (machine: Machine, forScaleOut: boolean) => {
      const newComps = { ...comps }
      const belongedComps = Object.values(comps).filter(
        (c) => c.machineID === machine.id
      )
      for (const c of belongedComps) {
        if (c.for_scale_out === forScaleOut) {
          delete newComps[c.id]
        }
      }
      setCompObjs(newComps)
    },
    [comps, setCompObjs]
  )

  function handleDeploy(values: any) {
    const topoYaml = yaml.stringify(genTopo(machines, comps, forScaleOut))
    deployCluster({
      ...values,
      topo_yaml: topoYaml,
      global_login_options: globalLoginOptions,
    })
    navigate('/status')
  }

  function handleScaleOut() {
    // save in local
    // cluster name, scale out nodes
    const scaleOutComps = Object.values(comps).filter(
      (comp) => comp.for_scale_out
    )
    if (scaleOutComps.length === 0) {
      Modal.warn({
        title: '扩容无法进行',
        content: '没有可扩容的组件',
      })
      return
    }

    const scaleOutNodes: any[] = []
    for (const comp of scaleOutComps) {
      let port: number = comp.symbolPort()
      const machine = machines[comp.machineID]
      scaleOutNodes.push({ id: comp.id, node: `${machine.host}:${port}` })
    }
    setCurScaleOutNodes({
      cluster_name: clusterName!,
      scale_out_nodes: scaleOutNodes,
    })

    // scale out
    const topoYaml = yaml.stringify(genTopo(machines, comps, forScaleOut))
    scaleOutCluster(clusterName!, {
      topo_yaml: topoYaml,
      global_login_options: globalLoginOptions,
    })
    navigate('/status')
  }

  function startOperate() {
    setPreviewYaml(false)

    if (forScaleOut) {
      handleScaleOut()
    } else {
      form.validateFields().then((values) => {
        setDeployReq(values as any)
        handleDeploy(values)
      })
    }
  }

  return (
    <Root>
      {forScaleOut ? (
        <Button type="primary" onClick={() => setPreviewYaml(true)}>
          预览扩容拓扑
        </Button>
      ) : (
        <Form form={form} layout="inline" initialValues={deployReq}>
          <Form.Item
            label="集群名字"
            name="cluster_name"
            rules={[{ required: true, message: '请输出集群名字' }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            label="TiDB 版本"
            name="tidb_version"
            rules={[{ required: true, message: '请选择 TiDB 版本' }]}
          >
            <Select style={{ width: 100 }}>
              {TIDB_VERSIONS.map((ver) => (
                <Select.Option key={ver} value={ver}>
                  {ver}
                </Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item>
            <Button type="primary" onClick={() => setPreviewYaml(true)}>
              预览部署拓扑
            </Button>
          </Form.Item>
        </Form>
      )}

      <div style={{ marginTop: 16 }}>
        <DeploymentTable
          forScaleOut={forScaleOut}
          machines={machines}
          components={comps}
          onAddComponent={handleAddComponent}
          onEditComponent={(rec) => setCurComp(rec)}
          onDeleteComponent={handleDeleteComponent}
          onDeleteComponents={handleDeleteComponents}
        />
      </div>

      <Drawer
        title={curComp && `修改 ${curComp.type} 组件`}
        width={400}
        closable={true}
        visible={curComp !== undefined}
        onClose={() => setCurComp(undefined)}
        destroyOnClose={true}
      >
        <EditCompForm comp={curComp} onUpdateComp={handleUpdateComponent} />
      </Drawer>

      <Modal
        title="Topology YAML"
        visible={previewYaml}
        okText={forScaleOut ? '开始扩容' : '开始部署'}
        cancelText="返回修改"
        onOk={startOperate}
        onCancel={() => setPreviewYaml(false)}
      >
        <TopoPreview
          machines={machines}
          components={comps}
          forScaleOut={forScaleOut}
        />
      </Modal>
    </Root>
  )
}
