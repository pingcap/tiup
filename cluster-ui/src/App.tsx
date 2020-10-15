import React from 'react'
import { HashRouter as Router, Routes, Route, Navigate } from 'react-router-dom'

import StatusPage from '_pages/Status'
import HomePage from '_pages/Home'
import MachinesPage from '_pages/Machines'
import DeploymentPage from '_pages/Deployment'
import ClustersPage from '_pages/Clusters'
import ClusterDetailPage from '_pages/Clusters/ClusterDetail'
import ClusterScaleOutPage from '_pages/Clusters/ClusterScaleOut'
import ClusterConfigPage from '_pages/Clusters/ClusterConfig'
import SettingPage from '_pages/Setting'

import './App.less'

function App() {
  return (
    <Router>
      <Routes>
        <Route path="/status" element={<StatusPage />} />

        <Route path="/" element={<HomePage />}>
          <Route path="" element={<Navigate to="/clusters" />} />
          <Route path="/clusters" element={<ClustersPage />}>
            <Route path=":clusterName" element={<ClusterDetailPage />} />
            <Route
              path=":clusterName/scaleout"
              element={<ClusterScaleOutPage />}
            />
            <Route path=":clusterName/config" element={<ClusterConfigPage />} />
          </Route>
          <Route path="/machines" element={<MachinesPage />} />
          <Route path="/deploy" element={<DeploymentPage />} />
          <Route path="/setting" element={<SettingPage />} />
        </Route>
      </Routes>
    </Router>
  )
}

export default App
