import { Button, Form, Input, message } from 'antd'
import React, { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { getMirrorAddress, setMirrorAddress } from '_apis'
import { Root } from '_components'
import { clearAuthToken } from '_utils/auth'

export default function SettingPage() {
  const [curMirrorAddress, setCurMirrorAddress] = useState('')
  const [refresh, setRefresh] = useState(0)
  const navigate = useNavigate()

  useEffect(() => {
    getMirrorAddress().then(({ data, err }) => {
      if (data) {
        const { mirror_address } = data
        setCurMirrorAddress(mirror_address)
      }
    })
  }, [refresh])

  function handleFinish(values: any) {
    setMirrorAddress(values.mirror_address).then(({ data, err }) => {
      if (err) {
        message.error(`修改失败，错误：${err.message}`)
      } else {
        message.success('修改成功')
        setRefresh((prev) => prev + 1)
      }
    })
  }

  function logOut() {
    clearAuthToken()
    navigate('/login')
  }

  return (
    <Root>
      <Form layout="vertical" onFinish={handleFinish}>
        <Form.Item label="镜像服务器地址或路径：" name="mirror_address">
          <Input style={{ width: 400 }} placeholder={curMirrorAddress} />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit">
            修改
          </Button>
        </Form.Item>
      </Form>
      <Button type="primary" htmlType="submit" onClick={logOut}>
        退出登录
      </Button>
    </Root>
  )
}
