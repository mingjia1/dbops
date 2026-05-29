import { Routes, Route, Navigate } from 'react-router-dom'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import InstanceList from './pages/InstanceList'
import EnvironmentCheck from './pages/EnvironmentCheck'
import BackupManage from './pages/BackupManage'
import MonitorDashboard from './pages/MonitorDashboard'

function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/dashboard" element={<Dashboard />} />
      <Route path="/instances" element={<InstanceList />} />
      <Route path="/env-check" element={<EnvironmentCheck />} />
      <Route path="/backup" element={<BackupManage />} />
      <Route path="/monitor" element={<MonitorDashboard />} />
      <Route path="/" element={<Navigate to="/login" replace />} />
    </Routes>
  )
}

export default App