import React, { useState, useEffect } from 'react'
import { Root } from '../../components/Root'
import OperationStatus, {
  IOperationStatus,
} from '../Deployment/OperationStatus'
import { getStatus } from '../../utils/api'

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
      {operationStatus ? (
        <OperationStatus operationStatus={operationStatus} />
      ) : (
        <div>Loading...</div>
      )}
    </Root>
  )
}
