import React from 'react'
import {
  BrowserRouter as Router,
  Routes,
  Route,
  NavLink,
  Navigate,
} from 'react-router-dom'
import { Layout, Menu } from 'antd'
import {
  HddOutlined,
  DeploymentUnitOutlined,
  ClusterOutlined,
} from '@ant-design/icons'

import MachinesPage from './pages/Machines'
import DeploymentPage from './pages/Deployment'

import './App.less'

const { Sider, Content } = Layout

function Clusters() {
  return <div>Clusters</div>
}

function SiderMenu() {
  return (
    <Sider collapsible>
      <Menu theme="dark" defaultSelectedKeys={['/machines']} mode="inline">
        <Menu.Item key="/machines" icon={<HddOutlined />}>
          <NavLink to="/machines">配置机器</NavLink>
        </Menu.Item>
        <Menu.Item key="/deploy" icon={<DeploymentUnitOutlined />}>
          <NavLink to="/deploy">部署</NavLink>
        </Menu.Item>
        <Menu.Item key="/clusters" icon={<ClusterOutlined />}>
          <NavLink to="/clusters">集群管理</NavLink>
        </Menu.Item>
      </Menu>
    </Sider>
  )
}

function App() {
  return (
    <Router>
      <Layout style={{ minHeight: '100vh' }}>
        <SiderMenu />
        <Content style={{ backgroundColor: 'white', padding: '24px' }}>
          <Routes>
            <Route path="/" element={<Navigate to="/machines" />} />
            <Route path="/machines" element={<MachinesPage />} />
            <Route path="/deploy" element={<DeploymentPage />} />
            <Route path="/clusters" element={<Clusters />} />
          </Routes>
        </Content>
      </Layout>
    </Router>
  )
}

export default App
