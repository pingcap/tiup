import React, { useEffect, useMemo, useState } from 'react'
import { Table, Typography } from 'antd'
import { getCheckClusterResult } from '_apis'
import { IClusterCheckResult } from '_types'

const { Text } = Typography

export default function CheckResult() {
  const [checkResults, setCheckResults] = useState<IClusterCheckResult[]>([])

  const columns = useMemo(() => {
    return [
      {
        title: 'Node',
        key: 'node',
        dataIndex: 'node',
      },
      {
        title: 'Name',
        key: 'name',
        dataIndex: 'name',
      },
      {
        title: 'Status',
        key: 'status',
        dataIndex: 'status',
        render: (text: any) => {
          if (text === 'Fail') {
            return <Text type="danger">{text}</Text>
          } else if (text === 'Warn') {
            return <Text type="warning">{text}</Text>
          } else {
            return <Text type="success">{text}</Text>
          }
        },
      },
      {
        title: 'Message',
        key: 'message',
        dataIndex: 'message',
      },
    ]
  }, [])

  useEffect(() => {
    const id = setInterval(function () {
      getCheckClusterResult('_any_').then(({ data, err }) => {
        if (data !== undefined && data.message !== 'checking') {
          setCheckResults(data)
          clearInterval(id)
        }
      })
    }, 1000)
    return () => clearInterval(id)
  }, [])

  return (
    <div>
      <p>检查结果：</p>
      <Table
        dataSource={checkResults}
        columns={columns}
        pagination={false}
        size="small"
        rowKey={(_rec, idx) => idx + ''}
      />
    </div>
  )
}
