import { Suspense, lazy, useEffect } from 'react'
import { App as AntApp, Spin } from 'antd'
import { Routes, Route, Navigate, useNavigate } from 'react-router-dom'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import ProtectedRoute from './components/ProtectedRoute'
import { onLogout } from './services/authEvents'

// P0: 20 个 page 静态 import 全部进 1 个 1.9MB chunk, 3G 首屏 5s 白屏.
// 改 lazy() 按需加载, 每个 page 独立 chunk. 总入口只 ~250KB.
const Home = lazy(() => import('./pages/Home'))
const HostList = lazy(() => import('./pages/HostList'))
const HostForm = lazy(() => import('./pages/HostForm'))
const HostDetail = lazy(() => import('./pages/HostDetail'))
const InstanceList = lazy(() => import('./pages/InstanceList'))
const InstanceDetail = lazy(() => import('./pages/InstanceDetail'))
const EnvironmentCheck = lazy(() => import('./pages/EnvironmentCheck'))
const BackupManage = lazy(() => import('./pages/BackupManage'))
const MonitorDashboard = lazy(() => import('./pages/MonitorDashboard'))
const ParameterTemplateList = lazy(() => import('./pages/ParameterTemplateList'))
const ApprovalManage = lazy(() => import('./pages/ApprovalManage'))
const AuditLog = lazy(() => import('./pages/AuditLog'))
const UpgradeManage = lazy(() => import('./pages/UpgradeManage'))
const AlertRuleList = lazy(() => import('./pages/AlertRuleList'))
const TopologyView = lazy(() => import('./pages/TopologyView'))
const MigrationManage = lazy(() => import('./pages/MigrationManage'))
const ClusterDeploy = lazy(() => import('./pages/ClusterDeploy'))
const DeployWizard = lazy(() => import('./pages/DeployWizard'))
const HAManage = lazy(() => import('./pages/HAManage'))
const RoleSwitch = lazy(() => import('./pages/RoleSwitch'))
const DataStorage = lazy(() => import('./pages/DataStorage'))
const AgentManage = lazy(() => import('./pages/AgentManage'))
const PluginManage = lazy(() => import('./pages/PluginManage'))
const SecuritySettings = lazy(() => import('./pages/SecuritySettings'))
const UserManagePage = lazy(() => import('./pages/UserManagePage'))

function App() {
  const navigate = useNavigate()

  useEffect(() => {
    return onLogout(() => {
      navigate('/login', { replace: true })
    })
  }, [navigate])

  return (
    // P1: 用 <AntApp> 包根, 让 Login / Dashboard 等页面的 AntApp.useApp()
    // 拿到正确的 message / modal / notification context (之前是 antd 静态 fallback).
    <AntApp>
      <Suspense fallback={<div style={{ padding: 32, textAlign: 'center' }}><Spin size="large" /></div>}>
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
            <Route path="deploy-wizard" element={<DeployWizard />} />
            <Route path="cluster-deploy" element={<ClusterDeploy />} />
            <Route path="ha" element={<HAManage />} />
            <Route path="role-switch" element={<RoleSwitch />} />
            <Route path="data-storage" element={<DataStorage />} />
            <Route path="agent-manage" element={<AgentManage />} />
            <Route path="plugins" element={<PluginManage />} />
            <Route path="security-settings" element={<SecuritySettings />} />
            <Route path="users" element={<UserManagePage />} />
          </Route>
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </Suspense>
    </AntApp>
  )
}

export default App
