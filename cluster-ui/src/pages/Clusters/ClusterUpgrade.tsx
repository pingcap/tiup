import React, { useMemo, useState, useEffect } from 'react'

import { useParams, useNavigate } from 'react-router-dom'
import { useSessionStorageState } from 'ahooks'
import { Button, Modal, Form, Select } from 'antd'

import { ICluster } from '_types'
import { Root } from '_components'
import { getTiDBVersions, upgradeCluster } from '_apis'

// extract
const TIDB_VERSIONS = [
  'nightly',
  'v4.0.11',
  'v4.0.10',
  'v4.0.9',
  'v4.0.8',
  'v4.0.7',
  'v4.0.6',
  'v4.0.5',
  'v4.0.4',
  'v4.0.3',
  'v4.0.2',
  'v4.0.1',
  'v4.0.0',
].map((v) => ({ value: v }))

function ClusterUpgradePage() {
  const [clustersList] = useSessionStorageState<ICluster[]>('clusters', [])
  const { clusterName } = useParams()
  const cluster = useMemo(
    () => clustersList.find((el) => el.name === clusterName),
    [clustersList, clusterName]
  )

  const [tidbVersions, setTiDBVersions] = useState(TIDB_VERSIONS)
  useEffect(() => {
    getTiDBVersions().then(({ data, err }) => {
      if (data !== undefined) {
        let versions = data.versions as string[]
        versions.reverse()
        const curVersionIdx = versions.indexOf(cluster!.version)
        if (curVersionIdx > 0) {
          versions = versions.slice(0, curVersionIdx + 1)
        }
        setTiDBVersions(['nightly'].concat(versions).map((v) => ({ value: v })))
      }
    })
  }, [cluster])

  const [form] = Form.useForm()

  const navigate = useNavigate()
  function handleUpgrade(values: any) {
    const { tidb_version } = values
    if (tidb_version === cluster?.version) {
      Modal.error({
        content: '升级版本必须大于当前版本',
      })
      return
    }
    // start upgrade
    upgradeCluster(clusterName, tidb_version)
    navigate('/status')
  }

  return (
    <Root>
      <h1>升级 {clusterName}</h1>
      {cluster && (
        <div>
          <p>当前 TiDB 版本: {cluster.version}</p>
          <Form
            form={form}
            initialValues={{ tidb_version: cluster.version }}
            onFinish={handleUpgrade}
          >
            <Form.Item
              label="目标 TiDB 版本"
              name="tidb_version"
              rules={[{ required: true, message: '请选择 TiDB 版本' }]}
            >
              <Select defaultValue={cluster.version} style={{ width: 200 }}>
                {tidbVersions.map((v) => (
                  <Select.Option value={v.value} key={v.value}>
                    {v.value}
                  </Select.Option>
                ))}
              </Select>
            </Form.Item>
            <Form.Item>
              <Button type="primary" htmlType="submit">
                开始升级
              </Button>
            </Form.Item>
          </Form>
        </div>
      )}
    </Root>
  )
}

export default function ClusterUpgradePageWrapper() {
  const { clusterName } = useParams()
  return <ClusterUpgradePage key={clusterName} />
}
