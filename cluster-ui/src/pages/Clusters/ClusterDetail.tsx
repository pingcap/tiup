import React, { useMemo, useState, useCallback, useEffect, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Space, Button, Modal, Table, Popconfirm, Divider } from 'antd'
import { ExclamationCircleOutlined } from '@ant-design/icons'
import { useSessionStorageState, useLocalStorageState } from 'ahooks'

import {
  deleteCluster,
  getClusterTopo,
  startCluster,
  stopCluster,
  scaleInCluster,
  checkCluster,
} from '_apis'
import { Root } from '_components'
import { useComps } from '_hooks'
import { ICluster, IClusterInstInfo } from '_types'

function ClusterDetailPage() {
  const refIframe = useRef<HTMLIFrameElement>(null)

  const navigate = useNavigate()
  const [clustersList] = useSessionStorageState<ICluster[]>('clusters', [])
  const { clusterName } = useParams()
  const cluster = useMemo(
    () => clustersList.find((el) => el.name === clusterName),
    [clustersList, clusterName]
  )

  const [clusterInstInfos, setClusterInstInfos] = useSessionStorageState<
    IClusterInstInfo[]
  >(`${clusterName}_cluster_topo`, [])

  const dashboardPD = useMemo(() => {
    return clusterInstInfos.find(
      (el) =>
        el.role === 'pd' &&
        el.status.indexOf('UI') !== -1 &&
        el.status.indexOf('Up') !== -1
    )
  }, [clusterInstInfos])

  const [curScaleOutNodes] = useLocalStorageState<{
    cluster_name: string
    scale_out_nodes: any[]
  }>('cur_scale_out_nodes', { cluster_name: '', scale_out_nodes: [] })
  const { comps, setCompObjs } = useComps()

  const [loadingTopo, setLoadingTopo] = useState(false)

  const handleScaleInCluster = useCallback(
    (node: IClusterInstInfo) => {
      const lowerStatus = node.status.toLowerCase()
      const force =
        lowerStatus.indexOf('down') !== -1 ||
        lowerStatus.indexOf('inactive') !== -1

      scaleInCluster(clusterName, {
        nodes: [node.id],
        force,
      })
      navigate('/status')
    },
    [navigate, clusterName]
  )

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
      fixed: title === 'ID',
    }))
    _columns.push({
      title: '操作',
      key: 'action',
      width: 100,
      fixed: 'right',
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
            <a>缩容</a>
          </Popconfirm>
        )
      },
    } as any)
    return _columns
  }, [handleScaleInCluster])

  useEffect(() => {
    setLoadingTopo(true)
    getClusterTopo(clusterName).then(({ data, err }) => {
      setLoadingTopo(false)
      if (data !== undefined) {
        setClusterInstInfos(data)
        updateLocalTopo(data)
      }
    })
    // eslint-disable-next-line
  }, [])

  const [dashboardToken] = useSessionStorageState(
    `${clusterName}_dashboard_token`,
    ''
  )

  function updateLocalTopo(clusters: IClusterInstInfo[]) {
    if (
      curScaleOutNodes.cluster_name !== clusterName ||
      curScaleOutNodes.scale_out_nodes.length === 0
    ) {
      return
    }
    let newComps = { ...comps }
    for (const n of curScaleOutNodes.scale_out_nodes) {
      const exist = clusters.find((el) => el.id === n.node)
      if (exist && newComps[n.id]) {
        newComps[n.id].for_scale_out = false
      }
    }
    setCompObjs(newComps)
  }

  function destroyCluster() {
    deleteCluster(clusterName)
    navigate('/status')
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

  function handleStartCluster() {
    startCluster(clusterName)
    navigate('/status')
  }

  function handleStopCluster() {
    stopCluster(clusterName)
    navigate('/status')
  }

  function handleScaleOutCluster() {
    navigate(`/clusters/${clusterName}/scaleout`)
  }

  function handleCheckCluster() {
    checkCluster(clusterName)
    navigate('/status')
  }

  function handleUpgradeCluster() {
    Modal.confirm({
      title: `升级 ${cluster?.name} 集群`,
      icon: <ExclamationCircleOutlined />,
      content: '升级前先执行环境检查',
      okText: '开始',
      cancelText: '取消',
      onOk: () => handleCheckCluster(),
    })
  }

  function handleOpenDashboard(targetFeature: string) {
    if (dashboardPD === undefined) {
      Modal.error({
        title: '没有找到相应的 PD 节点',
        content:
          '请检查相应的 PD 节点是否工作正常，如果集群未启动，请先启动集群。',
      })
      return
    }
    if (targetFeature === 'full') {
      refIframe.current?.contentWindow?.postMessage(
        {
          token: dashboardToken,
          lang: 'zh',
          hideNav: false,
          redirectPath: '/diagnose',
        },
        '*'
      )
      window.open(`http://${dashboardPD.id}/dashboard/`, '_blank')
      return
    }

    navigate(
      `/clusters/${clusterName}/dashboard?pd=${dashboardPD.id}&tidb_version=${cluster?.version}&target=${targetFeature}`
    )
  }

  return (
    <Root>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          flexWrap: 'wrap',
        }}
      >
        <Space>
          <Popconfirm
            title="你确定要启动集群吗？"
            onConfirm={handleStartCluster}
            okText="启动"
            cancelText="取消"
          >
            <Button type="primary">启动集群</Button>
          </Popconfirm>
          <Popconfirm
            title="你确定要停止集群吗？"
            onConfirm={handleStopCluster}
            okText="停止"
            cancelText="取消"
          >
            <Button>停止集群</Button>
          </Popconfirm>
          <Button onClick={handleScaleOutCluster}>扩容</Button>
          <Button onClick={handleUpgradeCluster}>升级</Button>
          <Divider type="vertical" />
          <Button onClick={() => handleOpenDashboard('configuration')}>
            修改配置
          </Button>
          <Button onClick={() => handleOpenDashboard('data')}>数据管理</Button>
          <Button onClick={() => handleOpenDashboard('dbusers')}>
            用户管理
          </Button>
          <Button onClick={() => handleOpenDashboard('full')}>更多功能</Button>
        </Space>
        <Button danger onClick={handleDestroyCluster}>
          销毁集群
        </Button>
      </div>

      {cluster && (
        <div style={{ marginTop: 16 }}>
          {/* <p>Name: {cluster.name}</p>
          <p>User: {cluster.user}</p> */}
          <p>Version: {cluster.version}</p>
          {/* <p>Path: {cluster.path}</p>
          <p>PrivateKey: {cluster.private_key}</p> */}
        </div>
      )}

      <Table
        dataSource={clusterInstInfos}
        columns={columns}
        pagination={false}
        rowKey={'id'}
        loading={loadingTopo}
        scroll={{ x: true }}
      />

      <iframe
        ref={refIframe}
        frameBorder="none"
        title="TiDB Configuration"
        width="100%"
        height="0"
        src={`http://${dashboardPD?.id}/dashboard/#/portal`}
      />
    </Root>
  )
}

export default function ClusterDetailPageWrapper() {
  const { clusterName } = useParams()
  return <ClusterDetailPage key={clusterName} />
}
