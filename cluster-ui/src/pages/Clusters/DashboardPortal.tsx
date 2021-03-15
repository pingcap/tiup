import React, { useEffect, useRef, useState } from 'react'
import { Alert, Button, Form, Input, Modal, Space } from 'antd'
import { useParams } from 'react-router-dom'
import { useSessionStorageState } from 'ahooks'

import { Root } from '_components'
import { request } from '_apis'
import { useQueryParams } from '_hooks'

enum VerifyStatus {
  Start,
  OK,
  Fail,
}

const TITLES: any = {
  configuration: '修改配置',
  data: '数据管理',
  dbusers: '用户管理',
}

export default function DashboardPortalPage() {
  const refIframe = useRef<HTMLIFrameElement>(null)
  const { clusterName } = useParams()
  const { pd, tidb_version, target } = useQueryParams()
  const [verified, setVerified] = useState<VerifyStatus>(VerifyStatus.Start)
  const [dashboardToken, setDashboardToken] = useSessionStorageState(
    `${clusterName}_dashboard_token`,
    ''
  )
  const [iframeLoad, setIframeLoad] = useState(false)

  useEffect(() => {
    // we need to wait for the iframe to load content
    // https://stackoverflow.com/questions/1463581/wait-for-iframe-to-load-in-javascript
    refIframe.current?.addEventListener('load', function () {
      setIframeLoad(true)
    })
  }, [])

  useEffect(() => {
    if (iframeLoad && verified === VerifyStatus.OK) {
      refIframe.current?.contentWindow?.postMessage(
        {
          token: dashboardToken,
          lang: 'zh',
          hideNav: true,
          redirectPath: `/${target}`,
        },
        '*'
      )
    }
  }, [iframeLoad, verified, dashboardToken, target])

  useEffect(() => {
    if (dashboardToken === '') {
      setVerified(VerifyStatus.Fail)
      return
    }
    // check whether token is valid
    request(`http://${pd}/dashboard/api/info/whoami`, 'GET', undefined, {
      headers: {
        Authorization: `Bearer ${dashboardToken}`,
      },
    }).then(({ data, err }) => {
      if (err) {
        setDashboardToken('')
        setVerified(VerifyStatus.Fail)
      } else {
        checkFeatureEnabled(dashboardToken)
      }
    })
    // eslint-disable-next-line
  }, [])

  function loginTiDB(values: any) {
    const { tidb_user, tidb_pwd } = values
    // 要根据 tidb 版本进行判断，4.0.6 及以上用 type，以下用 is_tidb_auth
    // shit! 'v4.0.11' < 'v4.0.6'
    // FIX ME
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
          const { token } = data
          setDashboardToken(token)
          checkFeatureEnabled(token)
        }
      }
    )
  }

  function checkFeatureEnabled(token: string) {
    request(`http://${pd}/dashboard/api/configuration/all`, 'GET', undefined, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    }).then(({ data, err }) => {
      if (err) {
        const { response } = err
        if (response.status === 404) {
          Modal.error({
            title: '此功能在此 TiDB 版本上不存在',
            content: '请检查该 TiDB 版本是否包含此功能',
          })
        } else if (response.status === 403) {
          Modal.error({
            title: '此功能在此 TiDB 版本未开启',
            content: '请检查该 TiDB 版本是否已开启此功能',
          })
        } else {
          Modal.error({
            title: '遇到错误',
            content: err.message,
          })
        }
      } else {
        setVerified(VerifyStatus.OK)
      }
    })
  }

  return (
    <Root>
      <h1>{TITLES[target]}</h1>
      {verified === VerifyStatus.Fail && (
        <Space direction="vertical">
          <Alert
            message={`${TITLES[target]}是危险操作，请输入 TiDB 用户名及密码进行验证！`}
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
          visibility: verified === VerifyStatus.OK ? 'visible' : 'hidden',
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
