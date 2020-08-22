import React, { useMemo } from 'react'
import { Root } from '../../components/Root'
import { useSessionStorageState } from 'ahooks'
import { ICluster } from '.'
import { useParams } from 'react-router-dom'

export default function ClusterDetailPage() {
  const [clustersList] = useSessionStorageState<ICluster[]>('clusters', [])
  const { clusterName } = useParams()
  const cluster = useMemo(
    () => clustersList.find((el) => el.name === clusterName),
    [clustersList, clusterName]
  )

  return (
    <Root>
      {cluster && (
        <>
          <p>Name: {cluster.name}</p>
          <p>User: {cluster.user}</p>
          <p>Version: {cluster.version}</p>
          <p>Path: {cluster.path}</p>
          <p>PrivateKey: {cluster.private_key}</p>
        </>
      )}
    </Root>
  )
}
