import React, { useCallback, useState, useEffect } from 'react'
import { useLocalStorageState } from 'ahooks'
import {
  Drawer,
  Space,
  Button,
  Modal,
  Form,
  Input,
  Select,
} from 'antd'
import uniqid from 'uniqid'
import yaml from 'yaml'

import { IMachine } from '../Machines/MachineForm'
import DeploymentTable, {
  IComponent,
  COMPONENT_TYPES,
  DEF_TIDB_PORT,
  DEF_TIDB_STATUS_PORT,
  DEF_TIKV_PORT,
  DEF_TIKV_STATUS_PORT,
  DEF_TIFLASH_TCP_PORT,
  DEF_TIFLASH_HTTP_PORT,
  DEF_TIFLASH_SERVICE_PORT,
  DEF_TIFLASH_PROXY_PORT,
  DEF_TIFLASH_PROXY_STATUS_PORT,
  DEF_TIFLASH_METRICS_PORT,
  DEF_PD_PEER_PORT,
  DEF_PROM_PORT,
  DEF_GRAFANA_PORT,
  DEF_ALERT_CLUSTER_PORT,
} from './DeploymentTable'
import EditCompForm from './EditCompForm'
import TopoPreview, { genTopo } from './TopoPreview'
import { Root } from '../../components/Root'
import OperationStatus, { IOperationStatus } from './OperationStatus'
import { getStatus, deployCluster, scaleOutCluster } from '../../utils/api'
import { IGlobalLoginOptions } from '../Machines/GlobalLoginOptionsForm'
import { ExclamationCircleOutlined } from '@ant-design/icons'

// TODO: fetch from API
const TIDB_VERSIONS = [
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
  // topo_yaml:string
}

export default function DeploymentPage() {
  const [machines] = useLocalStorageState<{
    [key: string]: IMachine
  }>('machines', {})
  const [components, setComponents] = useLocalStorageState<{
    [key: string]: IComponent
  }>('components', {})
  const [curComp, setCurComp] = useState<IComponent | undefined>(undefined)

  const [previewYaml, setPreviewYaml] = useState({
    preview: false,
    forScaleOut: false,
  })

  const [viewDeployStatus, setViewDeployStatus] = useState(false)

  const [operationStatus, setOperationStatus] = useState<
    IOperationStatus | undefined
  >(undefined)

  const [reloadTimes, setReloadTimes] = useState(0)

  const [deployReq, setDeployReq] = useLocalStorageState<IDeployReq>(
    'deploy_req',
    { cluster_name: '', tidb_version: '' }
  )

  const [globalLoginOptions] = useLocalStorageState<IGlobalLoginOptions>(
    'global_login_options',
    {}
  )

  const [form] = Form.useForm()

  useEffect(() => {
    const id = setInterval(() => {
      getStatus().then(({ data }) => {
        if (data !== undefined) {
          setOperationStatus(data)
          if (
            data.total_progress === 100 ||
            data.err_msg ||
            data.cluster_name === ''
          ) {
            clearInterval(id)
          }
        }
      })
    }, 2000)
    return () => clearInterval(id)
  }, [reloadTimes])

  const handleAddComponent = useCallback(
    (machine: IMachine, componentType: string, forScaleOut: boolean) => {
      let comp: IComponent = {
        id: uniqid(),
        machineID: machine.id,
        type: componentType,
        for_scale_out: forScaleOut,
        priority: COMPONENT_TYPES.indexOf(componentType),
      }
      const existedSameComps = Object.values(components).filter(
        (comp) => comp.type === componentType && comp.machineID === machine.id
      )
      if (existedSameComps.length > 0) {
        const lastComp = existedSameComps[existedSameComps.length - 1] as any
        comp.deploy_dir_prefix = lastComp.deploy_dir_prefix
        comp.data_dir_prefix = lastComp.data_dir_prefix
        let newComp = comp as any
        switch (componentType) {
          case 'TiDB':
            newComp.port = (lastComp.port || DEF_TIDB_PORT) + 1
            newComp.status_port =
              (lastComp.status_port || DEF_TIDB_STATUS_PORT) + 1
            break
          case 'TiKV':
            newComp.port = (lastComp.port || DEF_TIKV_PORT) + 1
            newComp.status_port =
              (lastComp.status_port || DEF_TIKV_STATUS_PORT) + 1
            break
          case 'TiFlash':
            newComp.tcp_port = (lastComp.tcp_port || DEF_TIFLASH_TCP_PORT) + 1
            newComp.http_port =
              (lastComp.http_port || DEF_TIFLASH_HTTP_PORT) + 1
            newComp.flash_service_port =
              (lastComp.flash_service_port || DEF_TIFLASH_SERVICE_PORT) + 1
            newComp.flash_proxy_port =
              (lastComp.flash_proxy_port || DEF_TIFLASH_PROXY_PORT) + 1
            newComp.flash_proxy_status_port =
              (lastComp.flash_proxy_status_port ||
                DEF_TIFLASH_PROXY_STATUS_PORT) + 1
            newComp.metrics_port =
              (lastComp.metrics_port || DEF_TIFLASH_METRICS_PORT) + 1
            break
          case 'PD':
            newComp.client_port = (lastComp.peer_port || DEF_PD_PEER_PORT) + 1
            newComp.peer_port = (lastComp.peer_port || DEF_PD_PEER_PORT) + 2
            break
          case 'Prometheus':
            newComp.port = (lastComp.port || DEF_PROM_PORT) + 1
            break
          case 'Grafana':
            newComp.port = (lastComp.port || DEF_GRAFANA_PORT) + 2
            break
          case 'AlertManager':
            newComp.web_port =
              (lastComp.cluster_port || DEF_ALERT_CLUSTER_PORT) + 1
            newComp.cluster_port =
              (lastComp.cluster_port || DEF_ALERT_CLUSTER_PORT) + 2
            break
        }
        comp = newComp
      }
      setComponents({
        ...components,
        [comp.id]: comp,
      })
    },
    [components, setComponents]
  )

  const handleUpdateComponent = useCallback(
    (comp: IComponent) => {
      setComponents({
        ...components,
        [comp.id]: comp,
      })
      setCurComp(undefined)
    },
    [components, setComponents]
  )

  const handleDeleteComponent = useCallback(
    (comp: IComponent) => {
      const newComps = { ...components }
      delete newComps[comp.id]
      setComponents(newComps)
    },
    [components, setComponents]
  )

  const handleDeleteComponents = useCallback(
    (machine: IMachine) => {
      const newComps = { ...components }
      const belongedComps = Object.values(components).filter(
        (c) => c.machineID === machine.id
      )
      for (const c of belongedComps) {
        delete newComps[c.id]
      }
      setComponents(newComps)
    },
    [components, setComponents]
  )

  function handleFinish(values: any) {
    if (operationStatus !== undefined) {
      const { cluster_name, total_progress, err_msg } = operationStatus
      if (cluster_name !== '' && err_msg === '' && total_progress < 100) {
        Modal.error({
          title: '操作暂时无法进行',
          content:
            '当前有正在进行中的部署或扩容任务，请点击 "查看进度" 进行查看',
        })
        return
      }
    }

    const topoYaml = yaml.stringify(
      genTopo({ machines, components, forScaleOut: previewYaml.forScaleOut })
    )
    if (previewYaml.forScaleOut) {
      scaleOutCluster(values.cluster_name, {
        topo_yaml: topoYaml,
        global_login_options: globalLoginOptions,
      })
    } else {
      deployCluster({
        ...values,
        topo_yaml: topoYaml,
        global_login_options: globalLoginOptions,
      })
    }
    setOperationStatus(undefined)
    setReloadTimes((pre) => pre + 1)
    setViewDeployStatus(true)
  }

  function startOperate() {
    setPreviewYaml((prev) => ({ ...prev, preview: false }))
    form.validateFields().then((values) => {
      setDeployReq(values as any)
      if (previewYaml.forScaleOut) {
        Modal.confirm({
          title: `开始扩容`,
          icon: <ExclamationCircleOutlined />,
          content:
            '即将开始扩容，请确保在扩容之前已经集群已部署成功，否则扩容将失败！',
          okText: '扩容',
          cancelText: '取消',
          onOk: () => handleFinish(values),
        })
      } else {
        handleFinish(values)
      }
    })
  }

  return (
    <Root>
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
          <Space>
            <Button
              onClick={() =>
                setPreviewYaml({ preview: true, forScaleOut: false })
              }
              disabled={operationStatus === undefined}
            >
              预览部署拓扑
            </Button>
            <Button
              onClick={() =>
                setPreviewYaml({ preview: true, forScaleOut: true })
              }
              disabled={operationStatus === undefined}
            >
              预览扩容拓扑
            </Button>
            <Button onClick={() => setViewDeployStatus(true)}>查看进度</Button>
          </Space>
        </Form.Item>
      </Form>

      <div style={{ marginTop: 16 }}>
        <DeploymentTable
          machines={machines}
          components={components}
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
        visible={previewYaml.preview}
        okText={previewYaml.forScaleOut ? '开始扩容' : '开始部署'}
        onOk={startOperate}
        onCancel={() => setPreviewYaml((prev) => ({ ...prev, preview: false }))}
      >
        <TopoPreview
          machines={machines}
          components={components}
          forScaleOut={previewYaml.forScaleOut}
        />
      </Modal>

      <Modal
        title="进度"
        visible={viewDeployStatus}
        onOk={() => setViewDeployStatus(false)}
        onCancel={() => setViewDeployStatus(false)}
      >
        {operationStatus ? (
          <OperationStatus operationStatus={operationStatus} />
        ) : (
          <div>Loading...</div>
        )}
      </Modal>
    </Root>
  )
}
