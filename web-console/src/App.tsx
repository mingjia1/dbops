import { Routes, Route, Navigate } from 'react-router-dom'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
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

function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/dashboard" element={<Dashboard />} />
      <Route path="/instances" element={<InstanceList />} />
      <Route path="/instances/:id" element={<InstanceDetail />} />
      <Route path="/env-check" element={<EnvironmentCheck />} />
      <Route path="/backup" element={<BackupManage />} />
      <Route path="/monitor" element={<MonitorDashboard />} />
      <Route path="/parameter-templates" element={<ParameterTemplateList />} />
      <Route path="/approvals" element={<ApprovalManage />} />
      <Route path="/audit-logs" element={<AuditLog />} />
      <Route path="/upgrade" element={<UpgradeManage />} />
      <Route path="/alert-rules" element={<AlertRuleList />} />
      <Route path="/topology" element={<TopologyView />} />
      <Route path="/migration" element={<MigrationManage />} />
      <Route path="/" element={<Navigate to="/login" replace />} />
    </Routes>
  )
}

export default App