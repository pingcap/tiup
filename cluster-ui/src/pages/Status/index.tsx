import React, { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'

import { Root } from '_components'
import { getStatus } from '_apis'
import { IOperationStatus } from '_types'

import OperationStatus from './OperationStatus'
import CheckResult from './CheckResult'
import { Space } from 'antd'

export default function StatusPage() {
  const [operationStatus, setOperationStatus] = useState<
    IOperationStatus | undefined
  >(undefined)

  useEffect(() => {
    const id = setInterval(() => {
      getStatus().then(({ data }) => {
        if (data !== undefined) {
          setOperationStatus(data)
          if (
            data.total_progress === 100 ||
            data.err_msg ||
            data.cluster_name === ''
          ) {
            clearInterval(id)
          }
        }
      })
    }, 2000)
    return () => clearInterval(id)
  }, [])

  return (
    <Root>
      <div style={{ display: 'flex', justifyContent: 'center' }}>
        <div style={{ width: 800 }}>
          <h1>操作状态</h1>
          {operationStatus ? (
            <OperationStatus operationStatus={operationStatus} />
          ) : (
            <div>Loading...</div>
          )}
          {operationStatus &&
            operationStatus.total_progress === 100 &&
            operationStatus.operation_type.startsWith('check') && (
              <CheckResult />
            )}
          {operationStatus &&
            (operationStatus.total_progress === 100 ||
              operationStatus.err_msg ||
              operationStatus.cluster_name === '') && (
              <div style={{ marginTop: 16 }}>
                <Space>
                  {operationStatus.operation_type === 'destroy' && (
                    <Link to="/clusters">进入集群管理</Link>
                  )}
                  {operationStatus.operation_type !== 'destroy' &&
                    !operationStatus.operation_type.startsWith('check') && (
                      <Link to={`/clusters/${operationStatus.cluster_name}`}>
                        进入集群管理
                      </Link>
                    )}
                  {operationStatus.operation_type === 'check_upgrade' && (
                    <>
                      <Link to={`/clusters/${operationStatus.cluster_name}`}>
                        修复后再升级
                      </Link>
                      <Link
                        to={`/clusters/${operationStatus.cluster_name}/upgrade`}
                      >
                        继续升级
                      </Link>
                    </>
                  )}
                  {operationStatus.operation_type === 'check_downgrade' && (
                    <>
                      <Link to={`/clusters/${operationStatus.cluster_name}`}>
                        修复后再降级
                      </Link>
                      <Link
                        to={`/clusters/${operationStatus.cluster_name}/downgrade`}
                      >
                        继续降级
                      </Link>
                    </>
                  )}
                </Space>
              </div>
            )}
        </div>
      </div>
    </Root>
  )
}
