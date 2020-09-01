import React, { useMemo } from 'react'
import { Tag, Table, Menu, Dropdown, Space, Divider, Popconfirm } from 'antd'
import { DownOutlined } from '@ant-design/icons'

import {
  BaseComp,
  COMP_TYPES_ARR,
  CompTypes,
  DEF_UESRNAME,
  Machine,
  DEF_SSH_PORT,
  MachineMap,
  CompMap,
} from '_types'
import { useGlobalLoginOptions } from '_hooks'

interface IDeploymentTableProps {
  forScaleOut: boolean // deploy or scale out
  machines: MachineMap
  components: CompMap
  onAddComponent?: (
    machine: Machine,
    componentType: CompTypes,
    forScaleOut: boolean
  ) => void
  onEditComponent?: (comp: BaseComp) => void
  onDeleteComponent?: (comp: BaseComp) => void
  onDeleteComponents?: (machine: Machine, forScaleOut: boolean) => void
}

export default function DeploymentTable({
  forScaleOut,
  machines,
  components,
  onAddComponent,
  onEditComponent,
  onDeleteComponent,
  onDeleteComponents,
}: IDeploymentTableProps) {
  const { globalLoginOptions } = useGlobalLoginOptions()

  const dataSource = useMemo(() => {
    let machinesAndComps: (Machine | BaseComp)[] = []
    const sortedMachines = Object.values(machines).sort((a, b) =>
      a.host > b.host ? 1 : -1
    )
    for (const machine of sortedMachines) {
      machinesAndComps.push(machine)
      const respondComps = Object.values(components).filter((c) => {
        if (forScaleOut) {
          return c.machineID === machine.id
        } else {
          return c.machineID === machine.id && !c.for_scale_out
        }
      })
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
  }, [machines, components, forScaleOut])

  const columns = useMemo(() => {
    return [
      {
        title: '目标机器 / 组件',
        key: 'target_machine_component',
        render: (text: any, rec: any) => {
          if (rec.host) {
            return `${rec.name} (${rec.host})`
          }
          return (
            <div>
              --&nbsp;
              <Tag key={rec.type} color={rec.color}>
                {rec.type}
                {rec.for_scale_out ? ' (Scale)' : ''}
              </Tag>
            </div>
          )
        },
      },
      {
        title: '信息',
        key: 'information',
        render: (text: any, rec: any) => {
          if (rec.host) {
            return `SSH Port=${rec.ssh_port || DEF_SSH_PORT}, User=${
              rec.username || globalLoginOptions.username || DEF_UESRNAME
            }, DC=${rec.dc}, Rack=${rec.rack}`
          }
          return `Port=${rec.ports()}, Path=${rec.allPathsPrefix()}`
        },
      },
      {
        title: '操作',
        key: 'action',
        render: (text: any, rec: any) => {
          if (rec.host) {
            return (
              <Space>
                <Dropdown
                  overlay={
                    <Menu
                      onClick={(e) =>
                        onAddComponent &&
                        onAddComponent(rec, e.key as CompTypes, forScaleOut)
                      }
                    >
                      {COMP_TYPES_ARR.map((t) => (
                        <Menu.Item key={t}>{t}</Menu.Item>
                      ))}
                    </Menu>
                  }
                  trigger={['click']}
                >
                  <a onClick={(e) => e.preventDefault()}>
                    {forScaleOut ? '扩容' : '添加'}组件 <DownOutlined />
                  </a>
                </Dropdown>
                <Divider type="vertical" />
                <Popconfirm
                  title={`你确定要删除所有${
                    forScaleOut ? '扩容' : '部署'
                  }组件吗？`}
                  onConfirm={() =>
                    onDeleteComponents && onDeleteComponents(rec, forScaleOut)
                  }
                  okText="删除"
                  cancelText="取消"
                >
                  <a>删除{forScaleOut ? '扩容' : '部署'}组件</a>
                </Popconfirm>
              </Space>
            )
          }
          if (forScaleOut === rec.for_scale_out) {
            return (
              <Space>
                <a onClick={() => onEditComponent && onEditComponent(rec)}>
                  编辑
                </a>
                <Divider type="vertical" />
                <Popconfirm
                  title="你确定要删除这个组件吗？"
                  onConfirm={() => onDeleteComponent && onDeleteComponent(rec)}
                  okText="删除"
                  cancelText="取消"
                >
                  <a>删除</a>
                </Popconfirm>
              </Space>
            )
          }
          return null
        },
      },
    ]
  }, [
    onAddComponent,
    onEditComponent,
    onDeleteComponent,
    onDeleteComponents,
    globalLoginOptions.username,
    forScaleOut,
  ])

  return (
    <Table
      size="small"
      dataSource={dataSource}
      columns={columns}
      pagination={false}
      rowKey="id"
    />
  )
}
