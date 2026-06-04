import { useEffect } from 'react'
import { Routes, Route, Navigate, useNavigate } from 'react-router-dom'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Home from './pages/Home'
import InstanceList from './pages/InstanceList'
import InstanceDetail from './pages/InstanceDetail'
import EnvironmentCheck from './pages/EnvironmentCheck'
import BackupManage from './pages/BackupManage'
import MonitorDashboard from './pages/MonitorDashboard'
import ParameterTemplateList from './pages/ParameterTemplateList'
import ApprovalManage from './pages/ApprovalManage'
import AuditLog from './pages/AuditLog'
import UpgradeManage from './pages/UpgradeManage'
import AlertRuleList from './pages/AlertRuleList'
import TopologyView from './pages/TopologyView'
import MigrationManage from './pages/MigrationManage'
import HostList from './pages/HostList'
import HostDetail from './pages/HostDetail'
import HostForm from './pages/HostForm'
import ClusterDeploy from './pages/ClusterDeploy'
import HAManage from './pages/HAManage'
import RoleSwitch from './pages/RoleSwitch'
import ProtectedRoute from './components/ProtectedRoute'
import { onLogout } from './services/authEvents'

function App() {
  const navigate = useNavigate()

  useEffect(() => {
    return onLogout(() => {
      navigate('/login', { replace: true })
    })
  }, [navigate])

  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        path="/dashboard"
        element={
          <ProtectedRoute>
            <Dashboard />
          </ProtectedRoute>
        }
      >
        <Route index element={<Navigate to="/dashboard/home" replace />} />
        <Route path="home" element={<Home />} />
        <Route path="hosts" element={<HostList />} />
        <Route path="hosts/new" element={<HostForm />} />
        <Route path="hosts/:id" element={<HostDetail />} />
        <Route path="hosts/:id/edit" element={<HostForm />} />
        <Route path="instances" element={<InstanceList />} />
        <Route path="instances/:id" element={<InstanceDetail />} />
        <Route path="resources" element={<Navigate to="/dashboard/hosts" replace />} />
        <Route path="env-check" element={<EnvironmentCheck />} />
        <Route path="backup" element={<BackupManage />} />
        <Route path="monitor" element={<MonitorDashboard />} />
        <Route path="parameter-templates" element={<ParameterTemplateList />} />
        <Route path="approvals" element={<ApprovalManage />} />
        <Route path="audit-logs" element={<AuditLog />} />
        <Route path="upgrade" element={<UpgradeManage />} />
        <Route path="alert-rules" element={<AlertRuleList />} />
        <Route path="topology" element={<TopologyView />} />
        <Route path="migration" element={<MigrationManage />} />
        <Route path="cluster-deploy" element={<ClusterDeploy />} />
        <Route path="ha" element={<HAManage />} />
        <Route path="role-switch" element={<RoleSwitch />} />
      </Route>
      <Route path="/" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  )
}

export default App
