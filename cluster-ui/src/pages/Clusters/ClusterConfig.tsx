import React, { useRef, useState } from 'react'
import { Alert, Button, Form, Input, Modal, Space } from 'antd'

import { Root } from '_components'
import { request } from '_apis'
import { useQueryParams } from '_hooks'

export default function ClusterConfigPage() {
  const refIframe = useRef<HTMLIFrameElement>(null)
  const { pd, tidb_version } = useQueryParams()
  const [verified, setVerified] = useState(false)

  function loginTiDB(values: any) {
    const { tidb_user, tidb_pwd } = values
    // 要根据 tidb 版本进行判断，4.0.6 及以上用 type，以下用 is_tidb_auth
    let loginOptions
    if (tidb_version >= 'v4.0.6') {
      loginOptions = {
        username: tidb_user,
        password: tidb_pwd,
        type: 0,
      }
    } else {
      loginOptions = {
        username: tidb_user,
        password: tidb_pwd,
        is_tidb_auth: true,
      }
    }
    request(`http://${pd}/dashboard/api/user/login`, 'POST', loginOptions).then(
      ({ data, err }) => {
        if (err) {
          Modal.error({
            title: '验证失败',
            content: err.message,
          })
        } else {
          setVerified(true)
          const { token } = data
          const iframe = refIframe.current
          iframe &&
            iframe.contentWindow!.postMessage(
              {
                token,
                lang: 'zh',
                hideNav: true,
                redirectPath: '/statement',
              },
              '*'
            )
        }
      }
    )
  }

  return (
    <Root>
      {!verified && (
        <Space direction="vertical">
          <Alert
            message="修改配置是危险操作，请输入 TiDB 用户名及密码进行验证！"
            showIcon
            type="error"
          />
          <Form
            layout="inline"
            initialValues={{ tidb_user: 'root', tidb_pwd: '' }}
            onFinish={loginTiDB}
          >
            <Form.Item label="TiDB 用户名" name="tidb_user">
              <Input disabled />
            </Form.Item>
            <Form.Item label="TiDB 密码" name="tidb_pwd">
              <Input.Password autoComplete="new-password" />
            </Form.Item>
            <Form.Item>
              <Button type="primary" htmlType="submit">
                验证
              </Button>
            </Form.Item>
          </Form>
        </Space>
      )}

      <div
        style={{
          height: '90vh',
          visibility: verified ? 'visible' : 'hidden',
        }}
      >
        <iframe
          ref={refIframe}
          frameBorder="none"
          title="TiDB Configuration"
          width="100%"
          height="100%"
          src={`http://${pd}/dashboard/#/portal`}
        />
      </div>
    </Root>
  )
}
