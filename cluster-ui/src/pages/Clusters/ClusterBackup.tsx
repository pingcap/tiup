import React, { useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'

import { Root } from '_components'
import { IBackupModel, INextBackup } from '_types'
import { getBackupList, getNextBackup } from '_apis'
import { Button, Table } from 'antd'

function ClusterBackupPage() {
  const { clusterName } = useParams()
  const [nextBackup, setNextBackup] = useState<INextBackup | undefined>(
    undefined
  )

  const [backups, setBackups] = useState<IBackupModel[]>([])

  useEffect(() => {
    getNextBackup(clusterName).then(({ data, err }) => {
      if (data !== undefined) {
        setNextBackup(data)
      }
    })
  }, [])

  useEffect(() => {
    getBackupList(clusterName).then(({ data, err }) => {
      if (data !== undefined) {
        setBackups(data)
      }
    })
  }, [])

  const columns = useMemo(() => {
    return [
      {
        title: '备份时间',
        key: 'start_time',
        dataIndex: 'start_time',
        width: 260,
      },
      {
        title: '备份目录',
        key: 'folder',
        render: (text: any, rec: IBackupModel) => {
          return `${rec.folder}/${rec.sub_folder}`
        },
      },
      {
        title: '备份结果',
        key: 'result',
        dataIndex: 'status',
      },
      {
        title: '备注',
        key: 'message',
        dataIndex: 'message',
      },
      {
        title: '操作',
        key: 'action',
      },
    ]
  }, [])

  return (
    <Root>
      <h1>备份</h1>
      {nextBackup?.enable_backup === false && (
        <Button type="primary">开启备份</Button>
      )}
      {nextBackup?.enable_backup === true && (
        <Button type="primary">设置</Button>
      )}
      <div style={{ marginTop: 16 }}>
        <Table dataSource={backups} columns={columns} pagination={false} />
      </div>
    </Root>
  )
}

export default function ClusterBackupPageWrapper() {
  const { clusterName } = useParams()
  return <ClusterBackupPage key={clusterName} />
}
