import React from 'react'
import { Progress } from 'antd'

export interface IDeploymentStatus {
  cluster_name: string
  is_deploying: boolean
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
      <p>当前在正进行的部署集群名字：{cluster_name}</p>
      <p>部署结果: {result()}</p>
      <p>进度：</p>
      <Progress
        percent={total_progress}
        status={total_progress < 100 ? 'active' : 'success'}
      />
      {err_msg && <p>错误日志：{err_msg}</p>}
      <p>部署步骤：</p>
      {steps.map((step, idx) => (
        <p key={idx}>{step}</p>
      ))}
    </div>
  )
}
