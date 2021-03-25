import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'

import { Root } from '_components'
import { IBackupModel } from '_types'
import { deleteBackup, getBackupList, updateBackupSetting } from '_apis'
import {
  Button,
  Divider,
  Drawer,
  Form,
  Input,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Typography,
} from 'antd'
import { useQueryParams } from '_hooks'

const { Text } = Typography

const clocks = new Array(48).fill(1).map((_, idx) => {
  const dayMinutes = idx * 30
  return {
    val: dayMinutes,
    text: formatDayMinutes(dayMinutes),
  }
})

function formatDayMinutes(dayMinutes: number) {
  const hours = Math.floor(dayMinutes / 60)
  const remainMins = dayMinutes % 60
  const hoursStr = (hours + '').padStart(2, '0')
  const remainMinsStr = (remainMins + '').padEnd(2, '0')
  return `${hoursStr}:${remainMinsStr}`
}

function ClusterBackupPage() {
  const { clusterName } = useParams()

  const [backups, setBackups] = useState<IBackupModel[]>([])

  const [showBackupSetting, setShowBackupSetting] = useState(false)
  const [updating, setUpdating] = useState(false)

  const { test } = useQueryParams()

  const queryBackupList = useCallback(() => {
    getBackupList(clusterName).then(({ data, err }) => {
      if (data !== undefined) {
        setBackups(data)
      }
    })
  }, [clusterName])

  useEffect(() => {
    queryBackupList()
  }, [queryBackupList])

  const nextBackup = useMemo(() => {
    const next = backups.find((el) => el.status === 'not_start')
    if (next) {
      const nextClock = clocks.find((c) => c.val === next.day_minutes)
      if (nextClock === undefined) {
        clocks.push({
          val: next.day_minutes,
          text: formatDayMinutes(next.day_minutes),
        })
      }
    }
    return next
  }, [backups])

  const onDelete = useCallback(
    async (item: IBackupModel) => {
      try {
        await deleteBackup(clusterName, item.ID)
      } finally {
        queryBackupList()
      }
    },
    [clusterName, queryBackupList]
  )

  const columns = useMemo(() => {
    return [
      {
        title: '备份时间',
        key: 'start_time',
        dataIndex: 'start_time',
        width: 260,
        render: (text: any, _rec: IBackupModel) => {
          return text || '还未开始'
        },
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
        render: (text: any, _rec: IBackupModel) => {
          switch (text) {
            case 'not_start':
              return <Text>未开始</Text>
            case 'running':
              return <Text type="secondary">备份中...</Text>
            case 'success':
              return <Text type="success">备份成功</Text>
            case 'fail':
              return <Text type="danger">备份失败</Text>
          }
        },
      },
      {
        title: '操作',
        key: 'action',
        render: (text: any, rec: IBackupModel) =>
          rec.status === 'success' && (
            <Space>
              {rec.status === 'success' && (
                <>
                  <Popconfirm
                    title="你确定要恢复这个备份吗？"
                    onConfirm={() => onRestore(rec)}
                    okText="恢复"
                    cancelText="取消"
                  >
                    <a>恢复</a>
                  </Popconfirm>
                  <Divider type="vertical" />
                </>
              )}
              {(rec.status === 'success' || rec.status === 'fail') && (
                <Popconfirm
                  title="你确定要删除这个备份吗？"
                  onConfirm={() => onDelete(rec)}
                  okText="删除"
                  cancelText="取消"
                >
                  <a>删除</a>
                </Popconfirm>
              )}
            </Space>
          ),
      },
    ]
  }, [onDelete])

  async function handleSubmitSetting(vals: any) {
    try {
      setUpdating(true)
      await updateBackupSetting(clusterName, vals)
    } finally {
      setUpdating(false)
      setShowBackupSetting(false)
      queryBackupList()
    }
  }

  // async function onDelete(item: IBackupModel) {
  //   try {
  //     await deleteBackup(clusterName, item.ID)
  //   } finally {
  //     queryBackupList()
  //   }
  // }

  async function onRestore(item: IBackupModel) {
    // todo
  }

  async function updateBackuptime() {
    if (nextBackup === undefined) {
      return
    }
    const now = new Date()
    const hours = now.getHours()
    const minutes = now.getMinutes()
    handleSubmitSetting({
      enable: true,
      folder: nextBackup.folder,
      day_minutes: hours * 60 + minutes + 2,
    })
  }

  return (
    <Root>
      <h1>备份</h1>
      <Button type="primary" onClick={() => setShowBackupSetting(true)}>
        {nextBackup ? '备份设置' : '开启备份'}
      </Button>
      {test && <Button onClick={updateBackuptime}>修改备份时间</Button>}
      <div style={{ marginTop: 16 }}>
        <Table
          dataSource={backups}
          columns={columns}
          pagination={false}
          rowKey="ID"
        />
      </div>

      <Drawer
        title="备份设置"
        width={400}
        closable={true}
        visible={showBackupSetting}
        onClose={() => setShowBackupSetting(false)}
        destroyOnClose={true}
      >
        <Form
          layout="vertical"
          onFinish={handleSubmitSetting}
          initialValues={{
            enable: nextBackup !== undefined,
            folder: nextBackup?.folder,
            day_minutes: nextBackup?.day_minutes,
          }}
        >
          <Form.Item name="enable" valuePropName="checked" label="开启">
            <Switch />
          </Form.Item>
          <Form.Item
            noStyle
            shouldUpdate={(prev, cur) => prev.enable !== cur.enable}
          >
            {({ getFieldValue }) => {
              return (
                getFieldValue('enable') && (
                  <>
                    <Form.Item noStyle>
                      <Form.Item
                        name="folder"
                        label="备份目录"
                        style={{ marginBottom: 0 }}
                      >
                        <Input />
                      </Form.Item>
                      <p style={{ fontStyle: 'italic', fontSize: 12 }}>
                        确保 nfs server
                        已挂载到所有节点，且所有节点皆可读写此目录
                      </p>
                    </Form.Item>
                    <Form.Item name="day_minutes" label="备份时间">
                      <Select style={{ width: 200 }}>
                        {clocks.map((t) => (
                          <Select.Option value={t.val} key={t.val}>
                            {t.text}
                          </Select.Option>
                        ))}
                      </Select>
                    </Form.Item>
                    <Form.Item label="备份周期">每天</Form.Item>
                    <Form.Item label="备份模式">full</Form.Item>
                  </>
                )
              )
            }}
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={updating}>
                保存
              </Button>
              <Button onClick={() => setShowBackupSetting(false)}>取消</Button>
            </Space>
          </Form.Item>
        </Form>
      </Drawer>
    </Root>
  )
}

export default function ClusterBackupPageWrapper() {
  const { clusterName } = useParams()
  return <ClusterBackupPage key={clusterName} />
}
