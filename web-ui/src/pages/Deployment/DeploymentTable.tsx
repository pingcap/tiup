import React, { useMemo } from 'react'
import { Tag, Table, Menu, Dropdown, Space, Divider, Popconfirm } from 'antd'
import { DownOutlined } from '@ant-design/icons'

import {
  BaseComp,
  COMP_TYPES_ARR,
  CompTypes,
  Machine,
  MachineMap,
  CompMap,
} from '_types'
import { useGlobalLoginOptions } from '_hooks'

type MachineOrComp = Machine | BaseComp

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
    let machinesAndComps: MachineOrComp[] = []
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
        respondComps.sort((a, b) => {
          let delta
          delta = a.priority - b.priority
          if (delta === 0) {
            delta = a.symbolPort() - b.symbolPort()
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
        render: (text: any, rec: MachineOrComp) => {
          if (rec instanceof Machine) {
            return `${rec.fullMachineName(globalLoginOptions)} (${rec.host})`
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
        render: (text: any, rec: MachineOrComp) => {
          if (rec instanceof Machine) {
            return `SSH Port=${rec.port()}, User=${rec.userName(
              globalLoginOptions
            )}, DC=${rec.dc}, Rack=${rec.rack}`
          }
          return `Port=${rec.ports()}, Path=${rec.allPathsPrefix()}`
        },
      },
      {
        title: '操作',
        key: 'action',
        render: (text: any, rec: MachineOrComp) => {
          if (rec instanceof Machine) {
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
    globalLoginOptions,
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
