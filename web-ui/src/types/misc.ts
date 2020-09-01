export interface IOperationStatus {
  operation_type: string // deploy | start | stop | scaleIn | scaleOut | destroy
  cluster_name: string
  total_progress: number
  steps: string[]
  err_msg: string
}
