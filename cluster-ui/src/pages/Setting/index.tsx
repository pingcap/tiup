import { Button, Form, Input, message } from 'antd'
import React, { useEffect, useState } from 'react'
import { getMirrorAddress, setMirrorAddress } from '_apis'
import { Root } from '_components'

export default function SettingPage() {
  const [curMirrorAddress, setCurMirrorAddress] = useState('')
  const [refresh, setRefresh] = useState(0)

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

  return (
    <Root>
      <Form layout="vertical" onFinish={handleFinish}>
        <Form.Item label="镜像服务器地址：" name="mirror_address">
          <Input style={{ width: 300 }} placeholder={curMirrorAddress} />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit">
            修改
          </Button>
        </Form.Item>
      </Form>
    </Root>
  )
}
