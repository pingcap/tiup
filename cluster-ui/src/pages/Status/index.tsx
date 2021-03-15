import React, { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'

import { Root } from '_components'
import { getStatus } from '_apis'
import { IOperationStatus } from '_types'

import OperationStatus from './OperationStatus'
import CheckResult from './CheckResult'

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
            operationStatus.operation_type === 'check' && <CheckResult />}
          {operationStatus &&
            (operationStatus.total_progress === 100 ||
              operationStatus.err_msg ||
              operationStatus.cluster_name === '') && (
              <div style={{ marginTop: 16 }}>
                {operationStatus.operation_type === 'destroy' ? (
                  <Link to="/clusters">进入集群管理</Link>
                ) : (
                  <Link to={`/clusters/${operationStatus.cluster_name}`}>
                    进入集群管理
                  </Link>
                )}
              </div>
            )}
        </div>
      </div>
    </Root>
  )
}
