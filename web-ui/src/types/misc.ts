export type OperationType =
  | 'deploy'
  | 'start'
  | 'stop'
  | 'scaleIn'
  | 'scaleOut'
  | 'destroy'

export interface IOperationStatus {
  operation_type: OperationType
  cluster_name: string
  total_progress: number
  steps: string[]
  err_msg: string
}
