import React, { useRef, useEffect } from 'react'
import { Progress } from 'antd'

export interface IDeploymentStatus {
  cluster_name: string
  total_progress: number
  steps: string[]
  err_msg: string
}

export interface IDeploymentStatusProps {
  deployStatus: IDeploymentStatus
}

export default function DeploymentStatus({
  deployStatus,
}: IDeploymentStatusProps) {
  const { cluster_name, total_progress, steps, err_msg } = deployStatus
  const detailInfoRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (detailInfoRef.current) {
      detailInfoRef.current.scrollTo(0, 36 * deployStatus.steps.length)
    }
  }, [deployStatus])

  function result() {
    if (err_msg) {
      return '失败'
    } else if (total_progress === 100) {
      return '成功'
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
