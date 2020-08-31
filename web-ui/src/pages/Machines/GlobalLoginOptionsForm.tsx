import React, { useState } from 'react'
import { Form, Input, Button, message } from 'antd'

export interface IGlobalLoginOptions {
  username?: string
  password?: string
  privateKey?: string
  privateKeyPassword?: string
}

export const DEF_UESRNAME = 'root'

export interface IGlobalLoginOptionsFormProps {
  globalLoginOptions: IGlobalLoginOptions
  onUpdateGlobalLoginOptions: (options: IGlobalLoginOptions) => void
}

export default function GlobalLoginOptionsForm({
  globalLoginOptions,
  onUpdateGlobalLoginOptions,
}: IGlobalLoginOptionsFormProps) {
  const [btnEnable, setBtnEnable] = useState(false)

  function handleFinish(values: any) {
    onUpdateGlobalLoginOptions(values)
    setBtnEnable(false)
    message.success('全局默认登录选项已修改')
  }

  return (
    <Form
      onValuesChange={() => setBtnEnable(true)}
      onFinish={handleFinish}
      layout="inline"
      title="全局默认登录选项"
      initialValues={globalLoginOptions}
    >
      <Form.Item label="登录用户名" name="username">
        <Input placeholder={DEF_UESRNAME} />
      </Form.Item>
      <Form.Item label="登录密码" name="password">
        <Input.Password />
      </Form.Item>
      <Form.Item label="私钥" name="privateKey" style={{ width: 400 }}>
        <Input.TextArea />
      </Form.Item>
      <Form.Item label="私钥密码" name="privateKeyPassword">
        <Input.Password />
      </Form.Item>
      <Form.Item>
        <Button type="primary" htmlType="submit" disabled={!btnEnable}>
          更新
        </Button>
      </Form.Item>
    </Form>
  )
}
