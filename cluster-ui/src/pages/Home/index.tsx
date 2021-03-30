import React, { useEffect, useState } from 'react'
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { Layout, Menu } from 'antd'
import {
  HddOutlined,
  DeploymentUnitOutlined,
  ClusterOutlined,
  SettingOutlined,
  AuditOutlined,
} from '@ant-design/icons'

import { getStatus } from '_apis'

const { Sider, Content } = Layout

function SiderMenu() {
  const [curMenu, setCurMenu] = useState('')

  // eslint-disable-next-line
  useEffect(() => {
    const path = window.location.hash.split('/')[1]
    setCurMenu(path || '')
  })

  return (
    <Sider collapsible>
      <Menu theme="dark" selectedKeys={[curMenu]} mode="inline">
        <Menu.Item key="clusters" icon={<ClusterOutlined />}>
          <NavLink to="/clusters">集群管理</NavLink>
        </Menu.Item>
        <Menu.Item key="machines" icon={<HddOutlined />}>
          <NavLink to="/machines">配置机器</NavLink>
        </Menu.Item>
        <Menu.Item key="deploy" icon={<DeploymentUnitOutlined />}>
          <NavLink to="/deploy">部署</NavLink>
        </Menu.Item>
        <Menu.Item key="audit" icon={<AuditOutlined />}>
          <NavLink to="/audit">审计日志</NavLink>
        </Menu.Item>
        <Menu.Item key="setting" icon={<SettingOutlined />}>
          <NavLink to="/setting">设置</NavLink>
        </Menu.Item>
      </Menu>
    </Sider>
  )
}

export default function HomePage() {
  const navigate = useNavigate()

  useEffect(() => {
    getStatus().then(({ data }) => {
      if (data !== undefined) {
        if (
          data.cluster_name !== '' &&
          data.total_progress < 100 &&
          data.err_msg === ''
        ) {
          navigate('/status')
        }
      }
    })
  }, [navigate])

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <SiderMenu />
      <Content style={{ backgroundColor: 'white' }}>
        <Outlet />
      </Content>
    </Layout>
  )
}