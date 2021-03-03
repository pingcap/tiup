import React, { useCallback, useEffect, useState } from 'react'
import { useLocalStorageState } from 'ahooks'
import { Drawer, Button, Modal, Form, Input, AutoComplete } from 'antd'
import yaml from 'yaml'
import { useNavigate } from 'react-router-dom'

import { Root } from '_components'
import { BaseComp, CompTypes, DEF_UESRNAME, Machine } from '_types'
import {
  useComps,
  useGlobalLoginOptions,
  useMachines,
  useGlobalDir,
} from '_hooks'
import { deployCluster, getTiDBVersions, scaleOutCluster } from '_apis'

import DeploymentTable from './DeploymentTable'
import EditCompForm from './EditCompForm'
import TopoPreview, { genTopo } from './TopoPreview'
import GlobalDirForm from './GlobalDirForm'

const TIDB_VERSIONS = [
  'nightly',
  'v4.0.11',
  'v4.0.10',
  'v4.0.9',
  'v4.0.8',
  'v4.0.7',
  'v4.0.6',
  'v4.0.5',
  'v4.0.4',
  'v4.0.3',
  'v4.0.2',
  'v4.0.1',
  'v4.0.0',
].map((v) => ({ value: v }))

export interface IDeployReq {
  cluster_name: string
  tidb_version: string
  mirror_address: string
}

export interface ICompsManagerProps {
  clusterName?: string
  forScaleOut: boolean
}

export default function CompsManager({
  clusterName,
  forScaleOut,
}: ICompsManagerProps) {
  const { globalLoginOptions } = useGlobalLoginOptions()
  const { machines } = useMachines()
  const { comps, setCompObjs } = useComps()

  const [curComp, setCurComp] = useState<BaseComp | undefined>(undefined)
  const [previewYaml, setPreviewYaml] = useState(false)

  const [deployReq, setDeployReq] = useLocalStorageState<IDeployReq>(
    'deploy_req',
    { cluster_name: '', tidb_version: '', mirror_address: '' }
  )
  const [, setCurScaleOutNodes] = useLocalStorageState(
    'cur_scale_out_nodes',
    {}
  )
  const { globalDir, setGlobalDirObj } = useGlobalDir()

  const [tidbVersions, setTiDBVersions] = useState(TIDB_VERSIONS)
  useEffect(() => {
    getTiDBVersions().then(({ data, err }) => {
      if (data !== undefined) {
        const versions = data.versions as string[]
        versions.reverse()
        setTiDBVersions(['nightly'].concat(versions).map((v) => ({ value: v })))
      }
    })
  }, [])

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

  const [topoYaml, setTopoYaml] = useState('')
  function handleDeploy(values: any) {
    deployCluster({
      ...values,
      topo_yaml: topoYaml,
      global_login_options: {
        ...globalLoginOptions,
        username: globalLoginOptions.username || DEF_UESRNAME,
      },
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
    const topoYaml = yaml.stringify(
      genTopo(globalDir, machines, comps, forScaleOut)
    )
    scaleOutCluster(clusterName!, {
      topo_yaml: topoYaml,
      global_login_options: globalLoginOptions,
    })
    navigate('/status')
  }

  function startOperate() {
    Modal.confirm({
      title: '请确保关键组件都已添加',
      cancelText: '再检查一下',
      okText: '开始',
      onOk: () => {
        setPreviewYaml(false)

        if (forScaleOut) {
          handleScaleOut()
        } else {
          form.validateFields().then((values) => {
            setDeployReq(values as any)
            handleDeploy(values)
          })
        }
      },
    })
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
            rules={[{ required: true, message: '请输入集群名字' }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            label="TiDB 版本"
            name="tidb_version"
            rules={[{ required: true, message: '请选择 TiDB 版本' }]}
          >
            <AutoComplete style={{ width: 200 }} options={tidbVersions} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" onClick={() => setPreviewYaml(true)}>
              预览部署拓扑
            </Button>
          </Form.Item>
        </Form>
      )}

      <div style={{ marginTop: 16 }}>
        <GlobalDirForm
          globalDir={globalDir}
          onUpdateGlobalDir={setGlobalDirObj}
        />
      </div>

      <div style={{ marginTop: 16 }}>
        <DeploymentTable
          forScaleOut={forScaleOut}
          machines={machines}
          components={comps}
          globalDir={globalDir}
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
        <EditCompForm
          globalDir={globalDir}
          comp={curComp}
          onUpdateComp={handleUpdateComponent}
        />
      </Drawer>

      <Modal
        title="Topology YAML"
        visible={previewYaml}
        okText={forScaleOut ? '开始扩容' : '开始部署'}
        cancelText="返回"
        onOk={startOperate}
        onCancel={() => setPreviewYaml(false)}
        destroyOnClose={true}
      >
        <TopoPreview
          globalDir={globalDir}
          machines={machines}
          components={comps}
          forScaleOut={forScaleOut}
          onTopoYamlChange={setTopoYaml}
        />
      </Modal>
    </Root>
  )
}
