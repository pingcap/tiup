import React from 'react'
import { useParams } from 'react-router-dom'
import CompsManager from '../Deployment/CompsManager'

export default function ClusterScaleOutPage() {
  const { clusterName } = useParams()

  return <CompsManager clusterName={clusterName} forScaleOut={true} />
}
