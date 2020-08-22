import React, { useMemo } from 'react'
import { Tag, Table, Menu, Dropdown, Space, Divider, Popconfirm } from 'antd'
import { DownOutlined } from '@ant-design/icons'

import { IMachine, DEF_SSH_PORT } from '../Machines/MachineForm'

export const COMPONENT_TYPES = [
  'TiDB',
  'TiKV',
  'TiFlash',
  'PD',
  'Prometheus',
  'Grafana',
  'AlertManager',
]

const COMPONENT_COLORS = {
  TiDB: 'magenta',
  TiKV: 'orange',
  TiFlash: 'red',
  PD: 'purple',
  Prometheus: 'green',
  Grafana: 'cyan',
  AlertManager: 'blue',
}

export interface IComponent {
  id: string
  machineID: string
  type: string
  priority: number

  deploy_dir_prefix?: string // "/tidb-deploy"
  data_dir_prefix?: string // "/tidb-data", TiDB and Grafana have no data_dir
}
export const DEF_DEPLOY_DIR_PREFIX = '/tidb-deploy'
export const DEF_DATA_DIR_PREFIX = '/tidb-data'

export interface ITiDBComponent extends IComponent {
  port?: number // 4000
  status_port?: number // 10080
}
export const DEF_TIDB_PORT = 4000
export const DEF_TIDB_STATUS_PORT = 10080

export interface ITiKVComponent extends IComponent {
  port?: number // 20160
  status_port?: number // 20180
}
export const DEF_TIKV_PORT = 20160
export const DEF_TIKV_STATUS_PORT = 20180

export interface ITiFlashComponent extends IComponent {
  tcp_port?: number // 9000
  http_port?: number // 8123
  flash_service_port?: number // 3930
  flash_proxy_port?: number // 20170
  flash_proxy_status_port?: number // 20292
  metrics_port?: number // 8234
}
export const DEF_TIFLASH_TCP_PORT = 9000
export const DEF_TIFLASH_HTTP_PORT = 8123
export const DEF_TIFLASH_SERVICE_PORT = 3930
export const DEF_TIFLASH_PROXY_PORT = 20170
export const DEF_TIFLASH_PROXY_STATUS_PORT = 20292
export const DEF_TIFLASH_METRICS_PORT = 8234

export interface IPDComponent extends IComponent {
  client_port?: number // 2379
  peer_port?: number // 2380
}
export const DEF_PD_CLIENT_PORT = 2379
export const DEF_PD_PEER_PORT = 2380

export interface IPrometheusComponent extends IComponent {
  port?: number // 9090
}
export const DEF_PROM_PORT = 9090

export interface IGrafanaComponent extends IComponent {
  port?: number // 3000
}
export const DEF_GRAFANA_PORT = 3000

export interface IAlertManagerComponent extends IComponent {
  web_port?: number // 9093
  cluster_port?: number // 9094
}
export const DEF_ALERT_WEB_PORT = 9093
export const DEF_ALERT_CLUSTER_PORT = 9094

interface IDeploymentTableProps {
  machines: { [key: string]: IMachine }
  components: { [key: string]: IComponent }
  onAddComponent?: (machine: IMachine, componentType: string) => void
  onEditComponent?: (comp: IComponent) => void
  onDeleteComponent?: (comp: IComponent) => void
  onDeleteComponents?: (machine: IMachine) => void
}

export default function DeploymentTable({
  machines,
  components,
  onAddComponent,
  onEditComponent,
  onDeleteComponent,
  onDeleteComponents,
}: IDeploymentTableProps) {
  const dataSource = useMemo(() => {
    let machinesAndComps: (IMachine | IComponent)[] = []
    const sortedMachines = Object.values(machines).sort((a, b) =>
      a.host > b.host ? 1 : -1
    )
    for (const machine of sortedMachines) {
      machinesAndComps.push(machine)
      const respondComps = Object.values(components).filter(
        (c) => c.machineID === machine.id
      )
      if (respondComps.length > 0) {
        respondComps.sort((a: any, b: any) => {
          let delta
          delta = a.priority - b.priority
          if (delta === 0) {
            delta =
              (a.port || a.tcp_port || a.client_port || a.web_port || 0) -
              (b.port || b.tcp_port || b.client_port || b.web_port || 0)
          }
          return delta
        })
        machinesAndComps.splice(machinesAndComps.length, 0, ...respondComps)
      }
    }
    return machinesAndComps
  }, [machines, components])

  const columns = useMemo(() => {
    return [
      {
        title: '目标机器 / 组件',
        key: 'target_machine_component',
        render: (text: any, rec: any) => {
          if (rec.username) {
            return `${rec.name} (${rec.host})`
          }
          return (
            <Tag key={rec.type} color={(COMPONENT_COLORS as any)[rec.type]}>
              {rec.type}
            </Tag>
          )
        },
      },
      {
        title: '信息',
        key: 'information',
        render: (text: any, rec: any) => {
          if (rec.username) {
            return `SSH Port=${rec.ssh_port || DEF_SSH_PORT}, User=${
              rec.username
            }, DC=${rec.dc}, Rack=${rec.rack}`
          }
          switch (rec.type) {
            case 'TiDB':
              return `Port=${rec.port || DEF_TIDB_PORT}/${
                rec.status_port || DEF_TIDB_STATUS_PORT
              }, Path=${rec.deploy_dir_prefix || DEF_DEPLOY_DIR_PREFIX}`
            case 'TiKV':
              return `Port=${rec.port || DEF_TIKV_PORT}/${
                rec.status_port || DEF_TIKV_STATUS_PORT
              }, Path=${rec.deploy_dir_prefix || DEF_DEPLOY_DIR_PREFIX},${
                rec.data_dir_prefix || DEF_DATA_DIR_PREFIX
              }`
            case 'TiFlash':
              return `Port=${rec.tcp_port || DEF_TIFLASH_TCP_PORT}/${
                rec.http_port || DEF_TIFLASH_HTTP_PORT
              }/${rec.flash_service_port || DEF_TIFLASH_SERVICE_PORT}/${
                rec.flash_proxy_port || DEF_TIFLASH_PROXY_PORT
              }/${
                rec.flash_proxy_status_port || DEF_TIFLASH_PROXY_STATUS_PORT
              }/${rec.metrics_port || DEF_TIFLASH_METRICS_PORT}, Path=${
                rec.deploy_dir_prefix || DEF_DEPLOY_DIR_PREFIX
              },${rec.data_dir_prefix || DEF_DATA_DIR_PREFIX}`
            case 'PD':
              return `Port=${rec.client_port || DEF_PD_CLIENT_PORT}/${
                rec.peer_port || DEF_PD_PEER_PORT
              }, Path=${rec.deploy_dir_prefix || DEF_DEPLOY_DIR_PREFIX},${
                rec.data_dir_prefix || DEF_DATA_DIR_PREFIX
              }`
            case 'Prometheus':
              return `Port=${rec.port || DEF_PROM_PORT}, Path=${
                rec.deploy_dir_prefix || DEF_DEPLOY_DIR_PREFIX
              },${rec.data_dir_prefix || DEF_DATA_DIR_PREFIX}`
            case 'Grafana':
              return `Port=${rec.port || DEF_GRAFANA_PORT}, Path=${
                rec.deploy_dir_prefix || DEF_DEPLOY_DIR_PREFIX
              }`
            case 'AlertManager':
              return `Port=${rec.web_port || DEF_ALERT_WEB_PORT}/${
                rec.cluster_port || DEF_ALERT_CLUSTER_PORT
              }, Path=${rec.deploy_dir_prefix || DEF_DEPLOY_DIR_PREFIX},${
                rec.data_dir_prefix || DEF_DATA_DIR_PREFIX
              }`
          }
          return '...'
        },
      },
      {
        title: '操作',
        key: 'action',
        render: (text: any, rec: any) => {
          if (rec.username) {
            return (
              <Space>
                <Dropdown
                  overlay={
                    <Menu
                      onClick={(e) =>
                        onAddComponent && onAddComponent(rec, e.key as string)
                      }
                    >
                      {COMPONENT_TYPES.map((t) => (
                        <Menu.Item key={t}>{t}</Menu.Item>
                      ))}
                    </Menu>
                  }
                  trigger={['click']}
                >
                  <a onClick={(e) => e.preventDefault()}>
                    添加组件 <DownOutlined />
                  </a>
                </Dropdown>
                <Divider type="vertical" />
                <Popconfirm
                  title="你确定要删除所有组件吗？"
                  onConfirm={() =>
                    onDeleteComponents && onDeleteComponents(rec)
                  }
                  okText="删除"
                  cancelText="取消"
                >
                  <a href="#">删除所有组件</a>
                </Popconfirm>
              </Space>
            )
          }
          return (
            <Space>
              <a
                href="#"
                onClick={() => onEditComponent && onEditComponent(rec)}
              >
                编辑
              </a>
              <Divider type="vertical" />
              <Popconfirm
                title="你确定要删除这个组件吗？"
                onConfirm={() => onDeleteComponent && onDeleteComponent(rec)}
                okText="删除"
                cancelText="取消"
              >
                <a href="#">删除</a>
              </Popconfirm>
            </Space>
          )
        },
      },
    ]
  }, [onAddComponent, onEditComponent, onDeleteComponent, onDeleteComponents])

  return (
    <Table
      size="middle"
      dataSource={dataSource}
      columns={columns}
      pagination={false}
      rowKey="id"
    />
  )
}
