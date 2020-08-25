import React, { useMemo, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Space, Button, Modal, message, Table, Popconfirm } from 'antd'
import { ExclamationCircleOutlined } from '@ant-design/icons'
import { useSessionStorageState } from 'ahooks'

import {
  deleteCluster,
  getClusterTopo,
  startCluster,
  stopCluster,
  scaleInCluster,
} from '../../utils/api'
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
  const [destroying, setDestroying] = useState(false)
  const [starting, setStarting] = useState(false)
  const [stoping, setStoping] = useState(false)
  const [scaleIning, setScaleIning] = useState(false)

  const [clusterInstInfos, setClusterInstInfos] = useSessionStorageState<
    IClusterInstInfo[]
  >(`${clusterName}_cluster_topo`, [])

  const [loadingTopo, setLoadingTopo] = useState(false)

  const columns = useMemo(() => {
    const _columns = [
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
    _columns.push({
      title: '操作',
      key: 'action',
      render: (text: any, rec: IClusterInstInfo) => {
        if (rec.status.toLowerCase().indexOf('offline') !== -1) {
          return null
        }
        return (
          <Popconfirm
            title="你确定要从集群中下线该组件吗？"
            onConfirm={() => handleScaleInCluster(rec)}
            okText="下线"
            cancelText="取消"
          >
            <a href="#">缩容</a>
          </Popconfirm>
        )
      },
    } as any)
    return _columns
  }, [])

  function destroyCluster() {
    setDestroying(true)
    deleteCluster(clusterName).then(({ data, err }) => {
      setDestroying(false)
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

  function handleStartCluster() {
    setStarting(true)
    startCluster(clusterName).then(({ data, err }) => {
      setStarting(false)
      handleShowTopo()
      if (data !== undefined) {
        message.success(`${clusterName} 集群已启动！`)
      } else if (err !== undefined) {
        Modal.confirm({
          title: `${clusterName} 集群启动失败`,
          content: err.message,
        })
      }
    })
  }

  function handleStopCluster() {
    setStoping(true)
    stopCluster(clusterName).then(({ data, err }) => {
      setStoping(false)
      handleShowTopo()
      if (data !== undefined) {
        message.success(`${clusterName} 集群已停止！`)
      } else if (err !== undefined) {
        Modal.confirm({
          title: `${clusterName} 集群停止失败`,
          content: err.message,
        })
      }
    })
  }

  function handleScaleInCluster(node: IClusterInstInfo) {
    const lowerStatus = node.status.toLowerCase()
    const force =
      lowerStatus.indexOf('down') !== -1 ||
      lowerStatus.indexOf('inactive') !== -1
    message.info(`${clusterName} 集群正在缩容中！请尽量不要进行其它操作！`)
    setScaleIning(true)

    scaleInCluster(clusterName, {
      node_id: node.id,
      force,
    }).then(({ data, err }) => {
      setScaleIning(false)
      handleShowTopo()
      if (data !== undefined) {
        message.success(`${clusterName} 集群缩容成功！`)
      } else if (err !== undefined) {
        Modal.confirm({
          title: `${clusterName} 集群缩容失败`,
          content: err.message,
        })
      }
    })
  }

  // function handleScaleOutCluster() {
  //   // TODO
  // }

  if (cluster === undefined) {
    return <Root></Root>
  }

  return (
    <Root>
      <div style={{ display: 'flex', justifyContent: 'space-between' }}>
        <Space>
          <Popconfirm
            title="你确定要启动集群吗？"
            onConfirm={handleStartCluster}
            okText="启动"
            cancelText="取消"
            disabled={destroying || stoping || scaleIning}
          >
            <Button
              type="primary"
              loading={starting}
              disabled={destroying || stoping || scaleIning}
            >
              启动集群
            </Button>
          </Popconfirm>
          <Popconfirm
            title="你确定要停止集群吗？"
            onConfirm={handleStopCluster}
            okText="停止"
            cancelText="取消"
            disabled={destroying || starting || scaleIning}
          >
            <Button
              loading={stoping}
              disabled={destroying || starting || scaleIning}
            >
              停止集群
            </Button>
          </Popconfirm>
        </Space>
        <Button
          danger
          onClick={handleDestroyCluster}
          loading={destroying}
          disabled={starting || stoping || scaleIning}
        >
          销毁群集
        </Button>
      </div>
      <div style={{ marginTop: 16 }}>
        <p>Name: {cluster.name}</p>
        <p>User: {cluster.user}</p>
        <p>Version: {cluster.version}</p>
        <p>Path: {cluster.path}</p>
        <p>PrivateKey: {cluster.private_key}</p>
      </div>

      <Space direction="vertical">
        <Button
          onClick={handleShowTopo}
          loading={loadingTopo}
          disabled={destroying || starting || stoping || scaleIning}
        >
          更新拓扑
        </Button>

        <Table
          dataSource={clusterInstInfos}
          columns={columns}
          pagination={false}
          rowKey={'id'}
          loading={
            loadingTopo || scaleIning || destroying || starting || stoping
          }
        />
      </Space>
    </Root>
  )
}
