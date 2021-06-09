import React from 'react'
import { Form, Input, Button } from 'antd'
import { useNavigate } from 'react-router-dom'

import { Root } from '_components'
import { login } from '_apis'
import { setAuthToken } from '_utils/auth'

const layout = {
  // labelCol: { span: 8 },
  // wrapperCol: { span: 16 },
}
const tailLayout = {
  // wrapperCol: { offset: 8, span: 16 },
}

export default function LoginPage() {
  const navigate = useNavigate()

  async function onFinish(values: any) {
    const { username, password } = values
    const { data, err } = await login(username, password)

    if (err === undefined) {
      const { token } = data
      setAuthToken(token)
      navigate('/')
    }
  }

  function onFinishFailed(errorInfo: any) {}

  return (
    <Root>
      <div style={{ maxWidth: 400, margin: '0 auto' }}>
        <h1 style={{ marginTop: 50 }}>欢迎使用 TiUP UI</h1>
        <Form
          {...layout}
          layout="vertical"
          name="basic"
          initialValues={{ remember: true }}
          onFinish={onFinish}
          onFinishFailed={onFinishFailed}
        >
          <Form.Item
            label="用户名"
            name="username"
            rules={[{ required: true, message: 'Please input your username!' }]}
          >
            <Input />
          </Form.Item>

          <Form.Item
            label="密码"
            name="password"
            rules={[{ required: true, message: 'Please input your password!' }]}
          >
            <Input.Password />
          </Form.Item>

          <Form.Item {...tailLayout}>
            <Button type="primary" htmlType="submit">
              登录
            </Button>
          </Form.Item>
        </Form>
      </div>
    </Root>
  )
}
