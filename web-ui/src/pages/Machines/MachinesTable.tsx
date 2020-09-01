import React, { useMemo } from 'react'
import { Table, Space, Divider, Popconfirm } from 'antd'

import { IGlobalLoginOptions, Machine } from '_types'

interface IMachinesTableProps {
  globalLoginOptions: IGlobalLoginOptions
  machines: { [key: string]: Machine }
  onEdit?: (m: Machine) => void
  onDelete?: (m: Machine) => void
}

export default function MachinesTable({
  globalLoginOptions,
  machines,
  onEdit,
  onDelete,
}: IMachinesTableProps) {
  const dataSource = useMemo(
    () => Object.values(machines).sort((a, b) => (a.host > b.host ? 1 : -1)),
    [machines]
  )
  const columns = useMemo(() => {
    return [
      {
        title: '机器名字',
        key: 'name',
        render: (text: any, rec: Machine) =>
          rec.fullMachineName(globalLoginOptions),
      },
      {
        title: '地址',
        key: 'address',
        render: (text: any, rec: Machine) => rec.address(),
      },
      {
        title: '登录用户',
        key: 'username',
        render: (text: any, rec: Machine) => rec.userName(globalLoginOptions),
      },
      {
        title: '使用公钥登录',
        key: 'isPubKeyAuth',
        render: (text: any, rec: any) => (rec.isPubKeyAuth ? '是' : '否'),
      },
      {
        title: '标签: DC',
        key: 'label_dc',
        dataIndex: 'dc',
      },
      {
        title: '标签: Rack',
        key: 'label_rack',
        dataIndex: 'rack',
      },
      {
        title: '操作',
        key: 'action',
        render: (text: any, rec: any) => (
          <Space>
            <a onClick={() => onEdit && onEdit(rec)}>编辑</a>
            <Divider type="vertical" />
            <Popconfirm
              title="你确定要删除这台主机吗？"
              onConfirm={() => onDelete && onDelete(rec)}
              okText="删除"
              cancelText="取消"
            >
              <a>删除</a>
            </Popconfirm>
          </Space>
        ),
      },
    ]
  }, [onEdit, onDelete, globalLoginOptions])

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
