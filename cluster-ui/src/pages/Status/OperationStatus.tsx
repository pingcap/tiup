import React from 'react'
import { Progress } from 'antd'

import { IOperationStatus } from '_types'

export interface IOperationStatusProps {
  operationStatus: IOperationStatus
}

export default function OperationStatus({
  operationStatus,
}: IOperationStatusProps) {
  const {
    operation_type,
    cluster_name,
    total_progress,
    steps,
    err_msg,
  } = operationStatus

  function result() {
    if (err_msg) {
      return '失败'
    } else if (total_progress === 100) {
      return '成功'
    } else {
      return '进行中'
    }
  }

  function progressBarStatus() {
    if (err_msg) {
      return 'exception'
    } else if (total_progress < 100) {
      return 'active'
    } else {
      return 'success'
    }
  }

  if (cluster_name === '') {
    return <p>当前没有正在进行的部署或扩容任务</p>
  }

  return (
    <div>
      <Progress percent={total_progress} status={progressBarStatus()} />
      <div style={{ marginTop: 16 }}>
        <p>正在执行的操作：{operation_type} cluster</p>
        <p>集群：{cluster_name}</p>
        <p>执行结果: {result()}</p>
        {err_msg && (
          <>
            <p>错误信息：</p>
            <p>{err_msg}</p>
          </>
        )}
        {steps.length > 0 && (
          <>
            <p>详细信息：</p>
            <div
              style={{
                maxHeight: 300,
                padding: 8,
                border: '1px solid #ccc',
                overflowY: 'auto',
              }}
            >
              {steps.map((step, idx) => (
                <p key={idx}>{step}</p>
              ))}
            </div>
          </>
        )}
      </div>
    </div>
  )
}
