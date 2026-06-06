import axios from 'axios'
import { message } from 'antd'
import { triggerLogout } from './authEvents'

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
})

api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

api.interceptors.response.use(
  (response) => {
    return response.data
  },
  (error) => {
    if (error.response?.status === 401) {
      // P1: 之前只 removeItem('token'), 'user' 残留导致 Dashboard 仍读出 user
      // 显示 "欢迎xxx", 业务上已经重定向到 /login 但前端 UI 不一致.
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      triggerLogout()
      return Promise.reject(error)
    }
    if (error.response?.status === 404) {
      return Promise.reject(error)
    }
    const errMsg = error.response?.data?.message || error.message || '请求失败'
    message.error(errMsg)
    return Promise.reject(error)
  }
)

export interface Instance {
  id: string
  name: string
  cluster_id: string
  host_id: string | null
  host?: string
  port?: number
  username?: string
  environment?: string
  ssl_enabled?: boolean
  created_at: string
  updated_at: string
  connection?: InstanceConnection
  version?: InstanceVersion
  status?: InstanceStatus
  topology?: InstanceTopology
}

export interface InstanceConnection {
  host: string
  port: number
  username: string
  ssl_enabled: boolean
  // Version-agnostic install / upgrade paths
  basedir?: string
  datadir?: string
  os_user?: string
  package_url?: string
  version_id?: string
}

export interface InstanceVersion {
  flavor: string
  version: string
  full_version: string
  is_lts: boolean
  eol_date: string
  features?: string
  engines?: string
}

export interface InstanceStatus {
  run_status: string
  health_status: string
  role: string
  replication_status?: string
  seconds_behind_master?: number
}

export interface InstanceTopology {
  cluster_id?: string
  master_id?: string
  slave_ids?: string
  replication_mode?: string
}

export const authApi = {
  login: (username: string, password: string) =>
    api.post('/auth/login', { username, password }),
  
  register: (username: string, password: string, email: string, role: string) =>
    api.post('/auth/register', { username, password, email, role }),
}

export const instanceApi = {
  list: (limit = 20, offset = 0) =>
    api.get(`/instances?limit=${limit}&offset=${offset}`),

  listByHost: (hostId: string, limit = 20, offset = 0) =>
    api.get(`/instances?host_id=${hostId}&limit=${limit}&offset=${offset}`),
  
  get: (id: string) =>
    api.get(`/instances/${id}`),
  
  create: (data: {
    name: string
    host: string
    port: number
    username: string
    password: string
    cluster_id?: string
    host_id?: string
    ssl_enabled?: boolean
  }) =>
    api.post('/instances', data),
  
  update: (id: string, data: { name?: string; cluster_id?: string; host_id?: string }) =>
    api.put(`/instances/${id}`, data),
  
  delete: (id: string) =>
    api.delete(`/instances/${id}`),
  
  detectVersion: (id: string) =>
    api.post(`/instances/${id}/detect-version`),

  deploy: (id: string) =>
    api.post(`/instances/${id}/deploy`),
}

export interface Host {
  id: string
  name: string
  address: string
  ssh_port: number
  ssh_user: string
  ssh_auth_method: string
  os_type: string
  description: string
  tags: string
  status: string
  last_check_at: string | null
  created_at: string
  updated_at: string
}

export interface HostTestResult {
  task_id: string
  host_id: string
  status: 'pending' | 'success' | 'failed'
  message: string
  latency_ms: number
  started_at: string
  ended_at: string
}

export const hostApi = {
  list: (limit = 20, offset = 0) =>
    api.get(`/hosts?limit=${limit}&offset=${offset}`),
  
  get: (id: string) =>
    api.get(`/hosts/${id}`),
  
  create: (data: {
    name: string
    address: string
    ssh_port?: number
    ssh_user: string
    ssh_auth_method?: string
    ssh_credential: string
    os_type?: string
    description?: string
    tags?: string
  }) =>
    api.post('/hosts', data),
  
  update: (id: string, data: {
    name?: string
    address?: string
    ssh_port?: number
    ssh_user?: string
    ssh_auth_method?: string
    ssh_credential?: string
    os_type?: string
    description?: string
    tags?: string
  }) =>
    api.put(`/hosts/${id}`, data),
  
  delete: (id: string) =>
    api.delete(`/hosts/${id}`),
  
  testConnection: (id: string) =>
    api.post(`/hosts/${id}/test`),

  getTestResult: (taskId: string) =>
    api.get(`/hosts/test/${taskId}`),

  scanInstances: (id: string, data?: { ports?: number[]; port_range?: string; probe_mysql?: boolean }) =>
    api.post(`/hosts/${id}/scan-instances`, data || {}),

  getScanResult: (hostId: string, taskId: string) =>
    api.get(`/hosts/${hostId}/scan-instances/${taskId}`),

  registerScannedInstance: (hostId: string, data: { port: number; name: string; username: string; password: string; cluster_id?: string }) =>
    api.post(`/hosts/${hostId}/scan-instances/register`, data),
}

export interface ScannedInstance {
  port: number
  version?: string
  version_full?: string
  flavor?: string
  role?: string
  datadir?: string
  socket?: string
  config_path?: string
  running: boolean
  pid?: number
  memory_mb?: number
  data_size_mb?: number
  recommended_name?: string
  already_managed?: boolean
  managed_instance_id?: string
}

export interface HostScanResult {
  task_id: string
  host_id: string
  status: 'pending' | 'running' | 'success' | 'failed'
  message: string
  instances: ScannedInstance[]
  scanned_at?: string
  error?: string
}

export const envCheckApi = {
  execute: (data: { hosts: { host: string; port: number; username: string; password: string }[] }) =>
    api.post('/env-checks', data),
  
  get: (checkId: string) =>
    api.get(`/env-checks/${checkId}`),
  
  export: (checkId: string, format = 'json') =>
    api.get(`/env-checks/${checkId}/export?format=${format}`),
}

export const backupApi = {
  createPolicy: (data: {
    instance_id: string
    backup_type: string
    schedule: string
    retention_days?: number
    storage_type?: string
    storage_path?: string
    enabled?: boolean
  }) =>
    api.post('/backups/policies', data),

  listPolicies: (instanceId?: string) =>
    api.get(`/backups/policies${instanceId ? `?instance_id=${instanceId}` : ''}`),

  updatePolicy: (id: string, data: any) =>
    api.put(`/backups/policies/${id}`, data),

  deletePolicy: (id: string) =>
    api.delete(`/backups/policies/${id}`),

  executeBackup: (instanceId: string, backupType: string) =>
    api.post('/backups', { instance_id: instanceId, backup_type: backupType }),

  listBackups: (instanceId: string) =>
    api.get(`/backups?instance_id=${instanceId}`),

  restore: (data: { backup_id: string; target_instance_id: string; target_type?: string; confirm_overwrite?: boolean }) =>
    api.post('/backups/restore', data),

  delete: (id: string) =>
    api.delete(`/backups/${id}`),

  scan: (instanceId: string) =>
    api.post(`/backups/scan`, { instance_id: instanceId }),
}

export interface DiscoveredBackup {
  file_name: string
  file_path: string
  size_bytes: number
  backup_type: string
  detected_at: string
  mtime?: string
  already_managed?: boolean
  managed_backup_id?: string
}

export const monitorApi = {
  queryMetrics: (instanceId: string) =>
    api.get(`/monitoring/metrics?instance_id=${instanceId}`),
}

export interface DataMigrationStatus {
  dialect: 'sqlite' | 'mysql'
  sqlite_path: string
  mysql_configured: boolean
  row_counts: Record<string, number>
}

export interface MigrateTableReport {
  table: string
  rows: number
  status: 'ok' | 'skipped' | 'failed'
  message: string
}

export interface MigrateResult {
  tables: MigrateTableReport[]
  total_rows: number
  duration_ms: number
  error?: string
}

export const dataMigrationApi = {
  getStatus: () => api.get<any, { code: number; data: DataMigrationStatus }>('/data-migration/status'),
  importLegacyJSON: () => api.post<any, { code: number; data: { imported: number } }>('/data-migration/import-legacy-json'),
  migrateToMySQL: () => api.post<any, { code: number; data?: MigrateResult; message: string }>('/data-migration/migrate-to-mysql'),
}

export interface ParameterTemplate {
  id: string
  name: string
  category: string
  description: string
  parameters: string
  is_preset: boolean
  created_by?: string
  created_at: string
  updated_at: string
}

export interface ApprovalRequest {
  id: string
  requester: string
  operation_type: string
  target_resource: string
  status: string
  description: string
  created_at: string
  updated_at: string
}

export interface AuditLog {
  id: string
  user: string
  action: string
  resource_type: string
  resource_id: string
  ip_address: string
  details: string
  created_at: string
}

export const parameterTemplateApi = {
  list: () =>
    api.get('/parameter-templates'),
  
  get: (id: string) =>
    api.get(`/parameter-templates/${id}`),
  
  create: (data: {
    name: string
    category: string
    description?: string
    parameters: string
  }) =>
    api.post('/parameter-templates', data),
  
  update: (id: string, data: {
    name?: string
    category?: string
    description?: string
    parameters?: string
  }) =>
    api.put(`/parameter-templates/${id}`, data),
  
  delete: (id: string) =>
    api.delete(`/parameter-templates/${id}`),

  recommend: (data: { instance_id?: string; template_id?: string; workload_type?: string }) =>
    api.post('/parameter-templates/recommend', data),

  apply: (data: { template_id: string; instance_id: string; parameters: string; require_restart?: boolean }) =>
    api.post('/parameter-templates/apply', data),
}

export const approvalApi = {
  list: (status?: string) =>
    api.get(`/approvals${status ? `?status=${status}` : ''}`),
  
  get: (id: string) =>
    api.get(`/approvals/${id}`),
  
  approve: (id: string, data: { comment?: string }) =>
    api.post(`/approvals/${id}/approve`, data),
  
  reject: (id: string, data: { reason: string }) =>
    api.post(`/approvals/${id}/reject`, data),
}

export const auditApi = {
  list: (filters?: {
    user?: string
    action?: string
    start_date?: string
    end_date?: string
  }) => {
    const params = new URLSearchParams()
    if (filters?.user) params.append('user', filters.user)
    if (filters?.action) params.append('action', filters.action)
    if (filters?.start_date) params.append('start_date', filters.start_date)
    if (filters?.end_date) params.append('end_date', filters.end_date)
    const queryString = params.toString()
    return api.get(`/audit-logs${queryString ? `?${queryString}` : ''}`)
  },

  get: (id: string) =>
    api.get(`/audit-logs/${id}`),
}

export const alertApi = {
  listRules: () => api.get('/alerts/rules'),
  getRule: (id: string) => api.get(`/alerts/rules/${id}`),
  createRule: (data: any) => api.post('/alerts/rules', data),
  updateRule: (id: string, data: any) => api.put(`/alerts/rules/${id}`, data),
  deleteRule: (id: string) => api.delete(`/alerts/rules/${id}`),
  listChannels: () => api.get('/alerts/notifications/channels'),
  createChannel: (data: any) => api.post('/alerts/notifications/channels', data),
  updateChannel: (id: string, data: any) => api.put(`/alerts/notifications/channels/${id}`, data),
  deleteChannel: (id: string) => api.delete(`/alerts/notifications/channels/${id}`),
  listHistory: () => api.get('/alerts/history'),
}

export const upgradeApi = {
  planPath: (data: any) => api.post('/upgrades/plan', data),
  checkCompat: (data: any) => api.post('/upgrades/check', data),
  executeInPlace: (data: any) => api.post('/upgrades/in-place', data),
  executeLogical: (data: any) => api.post('/upgrades/logical', data),
  executeRolling: (data: any) => api.post('/upgrades/rolling', data),
  listHistory: () => api.get('/upgrades'),
  getReport: (id: string) => api.get(`/upgrades/${id}/report`),
  get: (id: string) => api.get(`/upgrades/${id}`),
}

export interface VersionEntry {
  id: string
  flavor: string
  version: string
  major_minor: string
  is_lts: boolean
  release_date: string
  eol_date: string
  package_url: string
  checksum: string
  min_glibc: string
  os_family: string[]
  status: 'active' | 'deprecated' | 'eol'
  upgrade_from: string[]
  upgrade_notes?: string
}

export const versionApi = {
  // List all versions in the catalog. Optional ?flavor=mysql|mariadb|percona.
  list: (flavor?: string) => api.get('/versions' + (flavor ? `?flavor=${flavor}` : '')),
  get: (id: string) => api.get(`/versions/${encodeURIComponent(id)}`),
  // Validate an upgrade path (e.g. before submitting an in-place upgrade).
  validatePath: (data: { source_flavor?: string; source_version: string; target_flavor?: string; target_version: string }) =>
    api.post('/versions/validate-path', data),
}

export const migrationApi = {
  createPhysical: (data: any) => api.post('/migrations/physical', data),
  createReplication: (data: any) => api.post('/migrations/replication', data),
  createGTID: (data: any) => api.post('/migrations/gtid', data),
  verify: (taskId: string) => api.post(`/migrations/${taskId}/verify`),
  switchover: (taskId: string) => api.post(`/migrations/${taskId}/switch`),
  cancel: (taskId: string) => api.post(`/migrations/${taskId}/cancel`),
  list: () => api.get('/migrations'),
  get: (taskId: string) => api.get(`/migrations/${taskId}`),
}

export const clusterDeployApi = {
  deployMHA: (data: any) => api.post('/deployments/mha', data),
  deployMGR: (data: any) => api.post('/deployments/mgr', data),
  deployPXC: (data: any) => api.post('/deployments/pxc', data),
  getStatus: (id: string) => api.get(`/deployments/${id}`),
}

export const haApi = {
  healthCheck: (data: any) => api.post('/ha/health/batch', data),
  detectFailure: (instanceId: string) => api.get(`/ha/health/detect?instance_id=${instanceId}`),
  getFailureState: (instanceId: string) => api.get(`/ha/health/failure-state?instance_id=${instanceId}`),
  autoFailover: (data: any) => api.post('/ha/failover', data),
  manualSwitch: (data: any) => api.post('/ha/manual-switch', data),
  getStatus: (clusterId: string, limit = 10) =>
    api.get(`/ha/status?cluster_id=${clusterId}&limit=${limit}`),
}

export const roleSwitchApi = {
  switch: (data: { cluster_id: string; instance_id: string; target_role: string; old_master_id?: string }) =>
    api.post('/switch/cluster/role', data),
  history: (clusterId: string, limit = 20) =>
    api.get(`/switch/cluster/${clusterId}/role-history?limit=${limit}`),
}

export default api