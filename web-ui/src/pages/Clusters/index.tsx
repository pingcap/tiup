import React, { useState, useEffect } from 'react'
import { Layout, Menu } from 'antd'
import { NavLink } from 'react-router-dom'
import { getClusterList } from '../../utils/api'
import { Root } from '../../components/Root'

export default function ClustersPage() {
  const [clustersList, setClustersList] = useState<string[]>([])

  useEffect(() => {
    getClusterList().then((res) => {
      setClustersList(res.data || [])
    })
  }, [])

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Layout.Sider theme="light">
        <Menu>
          {clustersList.map((clsuter) => (
            <Menu.Item key={clsuter}>
              <NavLink to={`/clusters/${clsuter}`}>{clsuter}</NavLink>
            </Menu.Item>
          ))}
        </Menu>
      </Layout.Sider>
      <Layout.Content>
        <Root></Root>
      </Layout.Content>
    </Layout>
  )
}
