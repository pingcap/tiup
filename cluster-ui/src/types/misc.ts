export type OperationType =
  | 'deploy'
  | 'start'
  | 'stop'
  | 'scaleIn'
  | 'scaleOut'
  | 'destroy'
  | 'check'

export interface IOperationStatus {
  operation_type: OperationType
  cluster_name: string
  total_progress: number
  steps: string[]
  err_msg: string
}

export interface ICluster {
  name: string
  user: string
  version: string
  path: string
  private_key: string
}

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

export interface IClusterCheckResult {
  node: string
  name: string
  status: string
  message: string
}

export interface IAuditLogItem {
  id: string
  time: string
  command: string
}

export interface IBackupModel {
  plan_time: string
  start_time?: string
  folder: string
  sub_folder: string
  status: string
  message: string
}

export interface INextBackup {
  enable_backup: boolean
  next?: IBackupModel
}

export interface IBackupSetting {
  enable: boolean
  folder?: string
  day_minutes?: number
}
