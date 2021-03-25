import React, { useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'

import { Root } from '_components'
import { IBackupModel, INextBackup } from '_types'
import { getBackupList, getNextBackup, updateBackupSetting } from '_apis'
import { Button, Drawer, Form, Input, Select, Space, Switch, Table } from 'antd'

const clocks = new Array(48).fill(1).map((_, idx) => {
  const mins = idx * 30
  const hours = Math.floor(mins / 60)
  const remainMins = mins % 60
  const hoursStr = (hours + '').padStart(2, '0')
  const remainMinsStr = (remainMins + '').padEnd(2, '0')
  const text = `${hoursStr}:${remainMinsStr}`
  return {
    val: mins,
    text,
  }
})

function ClusterBackupPage() {
  const { clusterName } = useParams()
  const [nextBackup, setNextBackup] = useState<INextBackup | undefined>(
    undefined
  )

  const [backups, setBackups] = useState<IBackupModel[]>([])

  const [showBackupSetting, setShowBackupSetting] = useState(false)

  useEffect(() => {
    getNextBackup(clusterName).then(({ data, err }) => {
      if (data !== undefined) {
        setNextBackup(data)
      }
    })
  }, [clusterName])

  useEffect(() => {
    getBackupList(clusterName).then(({ data, err }) => {
      if (data !== undefined) {
        setBackups(data)
      }
    })
  }, [clusterName])

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

  function handleSubmitSetting(vals: any) {
    // console.log('vals:', vals)
    updateBackupSetting(clusterName, vals)
    setShowBackupSetting(false)
  }

  return (
    <Root>
      <h1>备份</h1>
      {nextBackup?.enable_backup === false && (
        <Button type="primary" onClick={() => setShowBackupSetting(true)}>
          开启备份
        </Button>
      )}
      {nextBackup?.enable_backup === true && (
        <Button type="primary" onClick={() => setShowBackupSetting(true)}>
          备份设置
        </Button>
      )}
      <div style={{ marginTop: 16 }}>
        <Table dataSource={backups} columns={columns} pagination={false} />
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
            enable: nextBackup?.enable_backup,
          }}
        >
          <Form.Item name="enable" valuePropName="checked" label="总开关">
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
              <Button type="primary" htmlType="submit">
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
