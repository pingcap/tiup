import React, { useEffect, useState } from 'react'
import { Layout, Menu, Space } from 'antd'
import { NavLink, Outlet, useNavigate, Link } from 'react-router-dom'
import { useSessionStorageState } from 'ahooks'

import { getClusterList } from '_apis'
import { Root } from '_components'
import { ICluster } from '_types'

export default function ClustersPage() {
  const [clustersList, setClustersList] = useSessionStorageState<ICluster[]>(
    'clusters',
    []
  )
  const navigate = useNavigate()
  const [curMenu, setCurMenu] = useState('')

  useEffect(() => {
    getClusterList().then((res) => {
      const clusters = res.data || []
      setClustersList(clusters)
      const paths = window.location.hash.split('/') // ['#', 'clusters', 'xxx']
      if (clusters.length === 1 && paths.length === 2) {
        navigate(`/clusters/${clusters[0].name}`)
      }
    })
    // eslint-disable-next-line
  }, [])

  useEffect(() => {
    const paths = window.location.hash.split('/') // ['#', 'clusters', 'xxx']
    if (paths[1] === 'clusters') {
      setCurMenu(paths[2] || '')
    }
  })

  if (clustersList.length === 0) {
    return (
      <Root>
        <p>当前没有可用的集群</p>
        <Space direction="vertical">
          <Link to="/machines">去配置机器</Link>
          <Link to="/deploy">去部署组件</Link>
        </Space>
      </Root>
    )
  }

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Layout.Sider theme="light">
        <Menu selectedKeys={[curMenu]}>
          {clustersList.map((cluster) => (
            <Menu.Item key={cluster.name}>
              <NavLink to={`/clusters/${cluster.name}`}>{cluster.name}</NavLink>
            </Menu.Item>
          ))}
        </Menu>
      </Layout.Sider>
      <Layout.Content>
        <Outlet />
      </Layout.Content>
    </Layout>
  )
}
