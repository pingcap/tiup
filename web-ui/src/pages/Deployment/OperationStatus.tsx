import React, { useRef } from 'react'
import { Progress } from 'antd'

export interface IOperationStatus {
  operation_type: string // 'deploy', 'scaleOut'
  cluster_name: string
  total_progress: number
  steps: string[]
  err_msg: string
}

export interface IOperationStatusProps {
  operationStatus: IOperationStatus
}

export default function OperationStatus({
  operationStatus,
}: IOperationStatusProps) {
  const { cluster_name, total_progress, steps, err_msg } = operationStatus
  const detailInfoRef = useRef<HTMLDivElement>(null)

  function result() {
    if (err_msg) {
      return '失败'
    } else if (total_progress === 100) {
      return '成功 (请进入 "集群管理" 界面对该集群进行启动，停止，缩容，销毁等操作)'
    } else {
      return '进行中'
    }
  }

  if (cluster_name === '') {
    return <p>当前没有正在进行的部署任务</p>
  }

  return (
    <div>
      <Progress
        percent={total_progress}
        status={total_progress < 100 ? 'active' : 'success'}
      />
      <div style={{ marginTop: 16 }}>
        <p>集群：{cluster_name}</p>
        <p>部署结果: {result()}</p>
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
              ref={detailInfoRef}
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
