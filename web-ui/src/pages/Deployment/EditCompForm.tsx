import React from 'react'
import { Form, Input, Button } from 'antd'

import {
  IComponent,
  DEF_DEPLOY_DIR_PREFIX,
  DEF_DATA_DIR_PREFIX,
  DEF_TIDB_PORT,
  DEF_TIDB_STATUS_PORT,
  DEF_TIKV_STATUS_PORT,
  DEF_TIKV_PORT,
  DEF_TIFLASH_TCP_PORT,
  DEF_TIFLASH_HTTP_PORT,
  DEF_TIFLASH_SERVICE_PORT,
  DEF_TIFLASH_PROXY_PORT,
  DEF_TIFLASH_PROXY_STATUS_PORT,
  DEF_TIFLASH_METRICS_PORT,
  DEF_PD_CLIENT_PORT,
  DEF_PD_PEER_PORT,
  DEF_GRAFANA_PORT,
  DEF_PROM_PORT,
  DEF_ALERT_WEB_PORT,
  DEF_ALERT_CLUSTER_PORT,
} from './DeploymentTable'

interface IEditCompFormProps {
  comp?: IComponent
  onUpdateComp: (comp: IComponent) => void
}

export default function EditCompForm({
  comp,
  onUpdateComp,
}: IEditCompFormProps) {
  function handleFinish(values: any) {
    console.log('before:', values)
    for (const key of Object.keys(values)) {
      let v = values[key]
      if (v === undefined) {
        continue
      }
      if (typeof v === 'number') {
        continue
      }
      if (typeof v === 'string') {
        v = v.trim()
      }
      if (v === '') {
        values[key] = undefined
        continue
      }
      if (key === 'deploy_dir_prefix' || key === 'data_dir_prefix') {
        continue
      }
      values[key] = parseInt(v)
      if (values[key] <= 0) {
        values[key] = undefined
      }
    }
    console.log('after:', values)
    onUpdateComp({
      ...comp,
      ...values,
    })
  }

  if (comp === undefined) {
    return null
  }

  const componentType = comp.type

  return (
    <Form layout="vertical" initialValues={comp} onFinish={handleFinish}>
      <Form.Item label="Deploy Dir Prefix" name="deploy_dir_prefix">
        <Input placeholder={DEF_DEPLOY_DIR_PREFIX} />
      </Form.Item>
      {['TiDB', 'Grafana'].indexOf(componentType) === -1 && (
        <Form.Item label="Data Dir Prefix" name="data_dir_prefix">
          <Input placeholder={DEF_DATA_DIR_PREFIX} />
        </Form.Item>
      )}
      {componentType === 'TiDB' && (
        <>
          <Form.Item label="Port" name="port">
            <Input placeholder={DEF_TIDB_PORT + ''} />
          </Form.Item>
          <Form.Item label="Status Port" name="status_port">
            <Input placeholder={DEF_TIDB_STATUS_PORT + ''} />
          </Form.Item>
        </>
      )}
      {componentType === 'TiKV' && (
        <>
          <Form.Item label="Port" name="port">
            <Input placeholder={DEF_TIKV_PORT + ''} />
          </Form.Item>
          <Form.Item label="Status Port" name="status_port">
            <Input placeholder={DEF_TIKV_STATUS_PORT + ''} />
          </Form.Item>
        </>
      )}
      {componentType === 'TiFlash' && (
        <>
          <Form.Item label="TCP Port" name="tcp_port">
            <Input placeholder={DEF_TIFLASH_TCP_PORT + ''} />
          </Form.Item>
          <Form.Item label="HTTP Port" name="http_port">
            <Input placeholder={DEF_TIFLASH_HTTP_PORT + ''} />
          </Form.Item>
          <Form.Item label="Flash Service Port" name="flash_service_port">
            <Input placeholder={DEF_TIFLASH_SERVICE_PORT + ''} />
          </Form.Item>
          <Form.Item label="Flash Proxy Port" name="flash_proxy_port">
            <Input placeholder={DEF_TIFLASH_PROXY_PORT + ''} />
          </Form.Item>
          <Form.Item
            label="Flash Proxy Status Port"
            name="flash_proxy_status_port"
          >
            <Input placeholder={DEF_TIFLASH_PROXY_STATUS_PORT + ''} />
          </Form.Item>
          <Form.Item label="Metrics Port" name="metrics_port">
            <Input placeholder={DEF_TIFLASH_METRICS_PORT + ''} />
          </Form.Item>
        </>
      )}
      {componentType === 'PD' && (
        <>
          <Form.Item label="Client Port" name="client_port">
            <Input placeholder={DEF_PD_CLIENT_PORT + ''} />
          </Form.Item>
          <Form.Item label="Peer Port" name="peer_port">
            <Input placeholder={DEF_PD_PEER_PORT + ''} />
          </Form.Item>
        </>
      )}
      {componentType === 'Prometheus' && (
        <Form.Item label="Port" name="port">
          <Input placeholder={DEF_PROM_PORT + ''} />
        </Form.Item>
      )}
      {componentType === 'Grafana' && (
        <Form.Item label="Port" name="port">
          <Input placeholder={DEF_GRAFANA_PORT + ''} />
        </Form.Item>
      )}
      {componentType === 'AlertManager' && (
        <>
          <Form.Item label="Web Port" name="web_port">
            <Input placeholder={DEF_ALERT_WEB_PORT + ''} />
          </Form.Item>
          <Form.Item label="Cluster Port" name="cluster_port">
            <Input placeholder={DEF_ALERT_CLUSTER_PORT + ''} />
          </Form.Item>
        </>
      )}
      <Form.Item>
        <Button type="primary" htmlType="submit">
          保存
        </Button>
      </Form.Item>
    </Form>
  )
}
