import React, { useEffect } from 'react'
import { Layout, Menu } from 'antd'
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useSessionStorageState } from 'ahooks'
import { getClusterList } from '../../utils/api'

export interface ICluster {
  name: string
  user: string
  version: string
  path: string
  private_key: string
}

export default function ClustersPage() {
  const [clustersList, setClustersList] = useSessionStorageState<ICluster[]>(
    'clusters',
    []
  )
  const navigate = useNavigate()

  useEffect(() => {
    getClusterList().then((res) => {
      const clusters = res.data || []
      setClustersList(clusters)
      if (clusters.length > 0) {
        navigate(`/clusters/${clusters[0].name}`)
      }
    })
    // eslint-disable-next-line
  }, [])

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Layout.Sider theme="light">
        <Menu>
          {clustersList.map((clsuter) => (
            <Menu.Item key={clsuter.name}>
              <NavLink to={`/clusters/${clsuter.name}`}>{clsuter.name}</NavLink>
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
