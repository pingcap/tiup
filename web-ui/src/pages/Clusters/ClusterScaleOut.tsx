import React, { useState } from 'react'
import { Root } from '../../components/Root'
import { useParams } from 'react-router-dom'

export default function ClusterScaleOutPage() {
  const { clusterName } = useParams()

  return <Root>ClusterScaleOutPage : {clusterName}</Root>
}
