import React, { useState } from 'react'
import { Form, Input, Button, message, Space } from 'antd'

import { GlobalDir, DEF_DEPLOY_DIR_PREFIX, DEF_DATA_DIR_PREFIX } from '_types'

export interface IGlobalDirFormProps {
  globalDir: GlobalDir
  onUpdateGlobalDir: (dirs: GlobalDir) => void
}

export default function GlobalDirForm({
  globalDir,
  onUpdateGlobalDir,
}: IGlobalDirFormProps) {
  const [btnEnable, setBtnEnable] = useState(false)

  function handleFinish(values: any) {
    onUpdateGlobalDir(values)
    setBtnEnable(false)
    message.success('全局默认目录已修改')
  }

  return (
    <Form
      onValuesChange={() => setBtnEnable(true)}
      onFinish={handleFinish}
      layout="inline"
      title="全局默认目录"
      initialValues={globalDir}
    >
      <Form.Item noStyle>
        <Space direction="vertical">
          <Form.Item
            label="Deploy Dir"
            name="deploy_dir_prefix"
            style={{ marginBottom: 0 }}
          >
            <Input placeholder={DEF_DEPLOY_DIR_PREFIX} />
          </Form.Item>
          <span style={{ paddingLeft: 80 }}>{globalDir.deployPathFull()}</span>
        </Space>
      </Form.Item>
      <Form.Item noStyle>
        <Space direction="vertical">
          <Form.Item
            label="Data Dir"
            name="data_dir_prefix"
            style={{ marginBottom: 0 }}
          >
            <Input placeholder={DEF_DATA_DIR_PREFIX} />
          </Form.Item>
          <span style={{ paddingLeft: 70 }}>{globalDir.dataPathFull()}</span>
        </Space>
      </Form.Item>
      <Form.Item>
        <Button type="primary" htmlType="submit" disabled={!btnEnable}>
          更新
        </Button>
      </Form.Item>
    </Form>
  )
}
