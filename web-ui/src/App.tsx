import React from 'react'
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'

import MachinesPage from './pages/Machines'
import DeploymentPage from './pages/Deployment'
import ClustersPage from './pages/Clusters'

import StatusPage from './pages/Status'
import HomePage from './pages/Home'
import ClusterScaleOutPage from './pages/Clusters/ClusterScaleOut'
import ClusterDetailPage from './pages/Clusters/ClusterDetail'

import './App.less'

function App() {
  return (
    <Router>
      <Routes>
        <Route path="/status" element={<StatusPage />} />

        <Route path="/" element={<HomePage />}>
          <Route path="/clusters" element={<ClustersPage />}>
            <Route path=":clusterName" element={<ClusterDetailPage />} />
            <Route
              path=":clusterName/scaleout"
              element={<ClusterScaleOutPage />}
            />
          </Route>
          <Route path="/machines" element={<MachinesPage />} />
          <Route path="/deploy" element={<DeploymentPage />} />
        </Route>
      </Routes>
    </Router>
  )
}

export default App
