import React, { useMemo, useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Space, Button, Modal, message, Table } from 'antd'
import { ExclamationCircleOutlined } from '@ant-design/icons'
import { useSessionStorageState } from 'ahooks'

import { deleteCluster, getClusterTopo } from '../../utils/api'
import { Root } from '../../components/Root'
import { ICluster } from '.'

export interface IClusterInstInfo {
  id: string
  role: string
  host: string
  ports: string
  os_arch: string
  status: string
  data_dir: string
  deploy_dir: string
}

export default function ClusterDetailPage() {
  const navigate = useNavigate()
  const [clustersList] = useSessionStorageState<ICluster[]>('clusters', [])
  const { clusterName } = useParams()
  const cluster = useMemo(
    () => clustersList.find((el) => el.name === clusterName),
    [clustersList, clusterName]
  )
  const [destroyingCluster, setDestroyingCluster] = useState(false)

  const [clusterInstInfos, setClusterInstInfos] = useSessionStorageState<
    IClusterInstInfo[]
  >(`${clusterName}_cluster_topo`, [])

  const [loadingTopo, setLoadingTopo] = useState(false)

  const columns = useMemo(() => {
    return [
      'ID',
      'Role',
      'Host',
      'Ports',
      'OS_Arch',
      'Status',
      'Data_Dir',
      'Deploy_Dir',
    ].map((title) => ({
      title,
      key: title.toLowerCase(),
      dataIndex: title.toLowerCase(),
    }))
  }, [])

  function destroyCluster() {
    setDestroyingCluster(true)
    deleteCluster(clusterName).then(({ data, err }) => {
      setDestroyingCluster(false)
      if (data) {
        message.success('销毁成功!')
        navigate('/')
      } else {
        message.error(err.message)
      }
    })
  }

  function handleDestroyCluster() {
    Modal.confirm({
      title: `销毁 ${cluster?.name} 集群`,
      icon: <ExclamationCircleOutlined />,
      content: '你确定要销毁这个集群吗？所有数据都会清除，操作不可回滚！',
      okText: '销毁',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: () => destroyCluster(),
    })
  }

  function handleShowTopo() {
    setLoadingTopo(true)
    getClusterTopo(clusterName).then(({ data, err }) => {
      setLoadingTopo(false)
      if (data !== undefined) {
        setClusterInstInfos(data)
      }
    })
  }

  if (cluster === undefined) {
    return <Root></Root>
  }

  return (
    <Root>
      <Space>
        <Button
          danger
          onClick={handleDestroyCluster}
          loading={destroyingCluster}
        >
          销毁群集
        </Button>
        <Button disabled>启动集群</Button>
      </Space>
      <div style={{ marginTop: 16 }}>
        <p>Name: {cluster.name}</p>
        <p>User: {cluster.user}</p>
        <p>Version: {cluster.version}</p>
        <p>Path: {cluster.path}</p>
        <p>PrivateKey: {cluster.private_key}</p>
      </div>

      <Space direction="vertical">
        <Button onClick={handleShowTopo} loading={loadingTopo}>
          更新拓扑
        </Button>

        <Table
          dataSource={clusterInstInfos}
          columns={columns}
          pagination={false}
          rowKey={'id'}
          loading={loadingTopo}
        />
      </Space>
    </Root>
  )
}
