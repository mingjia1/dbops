import axios from 'axios'
import { message } from 'antd'
import { triggerLogout } from './authEvents'

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
  // HttpOnly cookie (auth_token) is sent automatically; Authorization is no longer the only auth path.
  withCredentials: true,
})

const rejectBusinessError = (res: any) => {
  if (res && typeof res.code === 'number' && res.code !== 200) {
    return Promise.reject({
      response: { data: res },
      message: res.message || 'Request failed',
    })
  }
  return res
}

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
      // P1: clear both token and user so Dashboard does not render stale user data
      // after the request has already been redirected to /login.
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      triggerLogout()
      return Promise.reject(error)
    }
    if (error.response?.status === 404) {
      return Promise.reject(error)
    }
    const errMsg = error.response?.data?.message || error.message || 'Request failed'
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

  changePassword: (data: { current_password: string; new_password: string }) =>
    api.post('/auth/change-password', data),

  resetAllPasswords: (newPassword: string) =>
    api.post('/auth/reset-all-passwords', { new_password: newPassword }),
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
    version_id?: string
    package_url?: string
    basedir?: string
    datadir?: string
    os_user?: string
  }) =>
    api.post('/instances', data),

  batchCreate: (instances: Array<{
    name: string
    host: string
    port: number
    username: string
    password: string
    cluster_id?: string
    host_id?: string
    ssl_enabled?: boolean
    version_id?: string
    package_url?: string
    basedir?: string
    datadir?: string
    os_user?: string
  }>) =>
    api.post('/instances/batch', { instances }),
  
  update: (id: string, data: { name?: string; cluster_id?: string; host_id?: string }) =>
    api.put(`/instances/${id}`, data),
  
  delete: (id: string) =>
    api.delete(`/instances/${id}`),
  
  deploy: (id: string) =>
    api.post(`/instances/${id}/deploy`),

  healthCheck: (id: string) =>
    api.post(`/instances/${id}/health-check`, {}, { timeout: 30000 }).then(rejectBusinessError),

  adminAction: (id: string, data: {
    action: string
    username?: string
    user_host?: string
    password?: string
    privileges?: string
    scope?: string
    pattern?: string
    name?: string
    value?: string
    path?: string
    content?: string
    service?: string
    verb?: string
    update_stored_password?: boolean
  }) =>
    api.post(`/instances/${id}/admin-action`, data, { timeout: 120000 }),

  batchUpdatePassword: (data: {
    host: string
    ports: number[]
    username: string
    user_host?: string
    current_password?: string
    new_password: string
    update_stored?: boolean
  }) =>
    api.post('/instances/admin/batch-password', data),
}

export interface Host {
  id: string
  name: string
  address: string
  ssh_port: number
  ssh_user: string
  ssh_auth_method: string
  agent_port?: number
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
    agent_port?: number
    os_type?: string
    description?: string
    tags?: string
  }) =>
    api.post('/hosts', data),

  batchCreate: (hosts: Array<{
    name: string
    address: string
    ssh_port?: number
    ssh_user: string
    ssh_auth_method?: string
    ssh_credential: string
    agent_port?: number
    os_type?: string
    description?: string
    tags?: string
  }>) =>
    api.post('/hosts/batch', { hosts }),
  
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

  agentAction: (id: string, action: string, agentPort?: number) =>
    api.post(`/hosts/${id}/agent`, { action, agent_port: agentPort }, { timeout: 240000 }),

  batchAgentAction: (hostIds: string[], action: string, async = false, agentPort?: number, timeoutMs?: number) =>
    api.post('/hosts/agent/batch', { host_ids: hostIds, action, async, agent_port: agentPort }, { timeout: timeoutMs ?? (async ? 10000 : 240000) }),

  getTestResult: (taskId: string) =>
    api.get(`/hosts/test/${taskId}`),

  scanInstances: (id: string, data?: { ports?: number[]; port_range?: string; probe_mysql?: boolean }) =>
    api.post(`/hosts/${id}/scan-instances`, data || {}),

  getScanResult: (hostId: string, taskId: string) =>
    api.get(`/hosts/${hostId}/scan-instances/${taskId}`),

  registerScannedInstance: (hostId: string, data: { port: number; name: string; username: string; password: string; cluster_id?: string }) =>
    api.post(`/hosts/${hostId}/scan-instances/register`, data),

  registerScannedInstances: (hostId: string, instances: Array<{ port: number; name: string; username: string; password: string; cluster_id?: string }>) =>
    api.post(`/hosts/${hostId}/scan-instances/register-batch`, { instances }),
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
  execute: (data: { hosts?: { host: string; port: number; username: string; password: string }[]; host_ids?: string[] }) =>
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

  executeBackup: (instanceId: string, backupType: string, policyId?: string) =>
    api.post('/backups', { instance_id: instanceId, backup_type: backupType, policy_id: policyId }, { timeout: 300000 }),

  listBackups: (instanceId: string) =>
    api.get(`/backups?instance_id=${instanceId}`),

  restore: (data: { backup_id: string; target_instance_id: string; target_type?: string; confirm_overwrite?: boolean }) =>
    api.post('/backups/restore', data),

  delete: (id: string) =>
    api.delete(`/backups/${id}`),

  scan: (instanceId: string) =>
    api.post('/backups/scan', { instance_id: instanceId }, { timeout: 120000 }),
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
  parameters: ParameterTemplateParameter[]
  is_preset: boolean
  created_by?: string
  created_at: string
  updated_at: string
}

export interface ParameterTemplateParameter {
  id?: string
  template_id?: string
  version_id?: string
  parameter_name: string
  value: string
  data_type: string
  min_value?: string
  max_value?: string
  unit?: string
  description?: string
  is_dynamic?: boolean
  is_mandatory?: boolean
  category?: string
}

export interface ApprovalRequest {
  id: string
  requester: string
  requester_id?: string
  approver_id?: string
  operation_type: string
  target_resource: string
  resource_type?: string
  resource_id?: string
  status: string
  approval_status?: string
  description: string
  request_reason?: string
  approval_comment?: string
  priority?: number
  expires_at?: string
  created_at: string
  updated_at: string
}

export interface AuditLog {
  id: string
  user: string
  user_id?: string
  operation?: string
  action: string
  resource_type: string
  resource_id: string
  ip_address: string
  details: string
  result?: string
  error_msg?: string
  created_at: string
}

const inferParamType = (value: any): string => {
  const text = String(value)
  if (/^(on|off|true|false)$/i.test(text)) return 'bool'
  if (/^\d+(k|m|g|t)$/i.test(text)) return 'size'
  if (/^\d+$/.test(text)) return 'int'
  return 'string'
}

const parseTemplateParameters = (input: any): ParameterTemplateParameter[] => {
  if (Array.isArray(input)) return input
  if (!input || typeof input !== 'string') return []
  const parsed = JSON.parse(input)
  if (Array.isArray(parsed)) return parsed
  return Object.entries(parsed).map(([key, value]) => ({
    parameter_name: key,
    value: String(value),
    data_type: inferParamType(value),
    is_dynamic: true,
    category: 'custom',
  }))
}

export const parameterTemplateParamsToJson = (params: any): string => {
  const rows = parseTemplateParameters(params)
  const obj: Record<string, string> = {}
  rows.forEach((row) => {
    obj[row.parameter_name] = row.value
  })
  return JSON.stringify(obj, null, 2)
}

const normalizeParameterTemplate = (template: any): ParameterTemplate => ({
  ...template,
  parameters: parseTemplateParameters(template?.parameters),
})

const normalizeParameterTemplateResponse = (res: any) => {
  if (Array.isArray(res?.data)) {
    return { ...res, data: res.data.map(normalizeParameterTemplate) }
  }
  if (res?.data) return { ...res, data: normalizeParameterTemplate(res.data) }
  return res
}

const buildParameterTemplatePayload = (data: any) => ({
  ...data,
  parameters: parseTemplateParameters(data?.parameters),
})

const normalizeApproval = (item: any): ApprovalRequest => ({
  ...item,
  requester: item?.requester ?? item?.requester_id ?? '-',
  target_resource: item?.target_resource ?? ([item?.resource_type, item?.resource_id].filter(Boolean).join(':') || '-'),
  status: item?.status ?? item?.approval_status ?? 'pending',
  description: item?.description ?? item?.request_reason ?? '',
})

const normalizeApprovalResponse = (res: any) => {
  if (Array.isArray(res?.data)) return { ...res, data: res.data.map(normalizeApproval) }
  if (res?.data) return { ...res, data: normalizeApproval(res.data) }
  return res
}

const normalizeAuditLog = (item: any): AuditLog => ({
  ...item,
  user: item?.user ?? item?.user_id ?? '-',
  action: item?.action || item?.operation || '-',
})

const normalizeAuditResponse = (res: any) => {
  if (Array.isArray(res?.data)) return { ...res, data: res.data.map(normalizeAuditLog) }
  if (res?.data) return { ...res, data: normalizeAuditLog(res.data) }
  return res
}

export const parameterTemplateApi = {
  list: () =>
    api.get('/parameter-templates').then(normalizeParameterTemplateResponse),
  
  get: (id: string) =>
    api.get(`/parameter-templates/${id}`).then(normalizeParameterTemplateResponse),
  
  create: (data: {
    name: string
    category: string
    description?: string
    parameters: string
  }) =>
    api.post('/parameter-templates', buildParameterTemplatePayload(data)).then(normalizeParameterTemplateResponse),
  
  update: (id: string, data: {
    name?: string
    category?: string
    description?: string
    parameters?: string | ParameterTemplateParameter[]
  }) =>
    api.put(`/parameter-templates/${id}`, buildParameterTemplatePayload(data)).then(normalizeParameterTemplateResponse),
  
  delete: (id: string) =>
    api.delete(`/parameter-templates/${id}`),

  recommend: (data: { instance_id?: string; template_id?: string; workload_type?: string }) =>
    api.post('/parameter-templates/recommend', data),

  apply: (data: { template_id: string; instance_id: string; parameters?: string | ParameterTemplateParameter[]; require_restart?: boolean }) =>
    api.post('/parameter-templates/apply', { ...data, parameters: parseTemplateParameters(data.parameters) }),
}

export const approvalApi = {
  list: (status?: string) =>
    api.get(`/approvals${status ? `?status=${status}` : ''}`).then(normalizeApprovalResponse),
  
  get: (id: string) =>
    api.get(`/approvals/${id}`).then(normalizeApprovalResponse),

  create: (data: {
    requester_id: string
    operation_type: string
    resource_type: string
    resource_id: string
    request_reason?: string
    priority?: number
    expiry_hours?: number
  }) =>
    api.post('/approvals', data).then(normalizeApprovalResponse),
  
  approve: (id: string, data: { comment?: string }) =>
    api.post(`/approvals/${id}/approve`, data).then(normalizeApprovalResponse),
  
  reject: (id: string, data: { reason?: string; comment?: string }) =>
    api.post(`/approvals/${id}/reject`, { comment: data.comment ?? data.reason ?? '' }).then(normalizeApprovalResponse),
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
    return api.get(`/audit-logs${queryString ? `?${queryString}` : ''}`).then(normalizeAuditResponse)
  },

  get: (id: string) =>
    api.get(`/audit-logs/${id}`).then(normalizeAuditResponse),
}

const parseAlertChannels = (channels: any): string[] => {
  if (Array.isArray(channels)) return channels
  if (typeof channels !== 'string' || channels.trim() === '') return []
  try {
    const parsed = JSON.parse(channels)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return channels.split(',').map(item => item.trim()).filter(Boolean)
  }
}

const normalizeAlertRule = (rule: any) => ({
  ...rule,
  duration: rule?.duration ?? rule?.duration_seconds ?? 60,
  notification_channels: parseAlertChannels(rule?.notification_channels),
})

const normalizeAlertRuleResponse = (res: any) => {
  if (Array.isArray(res?.data)) {
    return { ...res, data: res.data.map(normalizeAlertRule) }
  }
  if (res?.data) {
    return { ...res, data: normalizeAlertRule(res.data) }
  }
  return res
}

const buildAlertRulePayload = (data: any) => {
  const { duration, duration_seconds, notification_channels, ...rest } = data || {}
  return {
    ...rest,
    duration_seconds: duration_seconds ?? duration ?? 60,
    notification_channels: JSON.stringify(parseAlertChannels(notification_channels)),
  }
}

export const alertApi = {
  listRules: async () => normalizeAlertRuleResponse(await api.get('/alerts/rules')),
  getRule: async (id: string) => normalizeAlertRuleResponse(await api.get(`/alerts/rules/${id}`)),
  createRule: async (data: any) => normalizeAlertRuleResponse(await api.post('/alerts/rules', buildAlertRulePayload(data))),
  updateRule: async (id: string, data: any) => normalizeAlertRuleResponse(await api.put(`/alerts/rules/${id}`, buildAlertRulePayload(data))),
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
  checksum_verified?: boolean
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
  list: (limit = 50, offset = 0) => api.get(`/deployments?limit=${limit}&offset=${offset}`),
  deployHA: (data: any) => api.post('/deployments/ha', data),
  deployMHA: (data: any) => api.post('/deployments/mha', data),
  deployMGR: (data: any) => api.post('/deployments/mgr', data),
  deployPXC: (data: any) => api.post('/deployments/pxc', data),
  getStatus: (id: string) => api.get(`/deployments/${id}`),
  destroy: (id: string) => api.delete(`/deployments/${id}`),
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

export const topologyApi = {
  getCluster: (clusterId: string) => api.get(`/topology/clusters/${encodeURIComponent(clusterId)}`),
  getGraph: (clusterId: string) => api.get(`/topology/clusters/${encodeURIComponent(clusterId)}/graph`),
  getInstance: (instanceId: string) => api.get(`/topology/instances/${encodeURIComponent(instanceId)}`),
}

export default api
