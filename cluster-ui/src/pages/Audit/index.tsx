import { Table } from 'antd'
import React, { useEffect, useMemo, useState } from 'react'
import { getAuditList } from '_apis'
import { Root } from '_components'
import { IAuditLogItem } from '_types'

export default function AuditPage() {
  const [auditList, setAuditList] = useState<IAuditLogItem[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    function queryAuditList() {
      setLoading(true)
      getAuditList().then(({ data, err }) => {
        if (data !== undefined) {
          setAuditList(data)
        }
        setLoading(false)
      })
    }
    queryAuditList()
  }, [])

  const columns = useMemo(() => {
    return [
      {
        title: '时间',
        key: 'time',
        dataIndex: 'time',
        width: 260,
      },
      {
        title: '操作',
        key: 'command',
        dataIndex: 'command',
      },
    ]
  }, [])

  return (
    <Root>
      <Table
        dataSource={auditList}
        columns={columns}
        rowKey="id"
        loading={loading}
      />
    </Root>
  )
}
