import React, { useMemo } from 'react'
import { Table, Space, Divider, Popconfirm } from 'antd'

import { IMachine, DEF_SSH_PORT } from './MachineForm'

interface IMachinesTableProps {
  machines: { [key: string]: IMachine }
  onEdit?: (m: IMachine) => void
  onDelete?: (m: IMachine) => void
}

export default function MachinesTable({
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
        dataIndex: 'name',
        key: 'name',
      },
      {
        title: '地址',
        key: 'address',
        render: (text: any, rec: any) =>
          `${rec.host}:${rec.ssh_port || DEF_SSH_PORT}`,
      },
      {
        title: '登录用户',
        dataIndex: 'username',
        key: 'username',
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
            <a href="#" onClick={() => onEdit && onEdit(rec)}>
              编辑
            </a>
            <Divider type="vertical" />
            <Popconfirm
              title="你确定要删除这台主机吗？"
              onConfirm={() => onDelete && onDelete(rec)}
              okText="删除"
              cancelText="取消"
            >
              <a href="#">删除</a>
            </Popconfirm>
          </Space>
        ),
      },
    ]
  }, [onEdit, onDelete])

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
