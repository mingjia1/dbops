import axios from 'axios'
import { message } from 'antd'

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
      localStorage.removeItem('token')
      window.location.href = '/login'
    }
    message.error(error.response?.data?.message || '请求失败')
    return Promise.reject(error)
  }
)

export interface Instance {
  id: string
  name: string
  cluster_id: string
  created_at: string
  updated_at: string
}

export interface InstanceConnection {
  host: string
  port: number
  username: string
  ssl_enabled: boolean
}

export interface InstanceVersion {
  flavor: string
  version: string
  full_version: string
  is_lts: boolean
  eol_date: string
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
  
  get: (id: string) =>
    api.get(`/instances/${id}`),
  
  create: (data: {
    name: string
    host: string
    port: number
    username: string
    password: string
    cluster_id?: string
    ssl_enabled?: boolean
  }) =>
    api.post('/instances', data),
  
  update: (id: string, data: { name?: string; cluster_id?: string }) =>
    api.put(`/instances/${id}`, data),
  
  delete: (id: string) =>
    api.delete(`/instances/${id}`),
  
  detectVersion: (id: string) =>
    api.post(`/instances/${id}/detect-version`),
}

export const envCheckApi = {
  execute: (hosts: { host: string; port: number; username: string; password: string }[]) =>
    api.post('/env-checks', { hosts }),
  
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
  
  executeBackup: (instanceId: string, backupType: string) =>
    api.post('/backups', { instance_id: instanceId, backup_type: backupType }),
  
  listBackups: (instanceId: string) =>
    api.get(`/backups?instance_id=${instanceId}`),
}

export const monitorApi = {
  queryMetrics: (instanceId: string) =>
    api.get(`/monitoring/metrics?instance_id=${instanceId}`),
}

export interface ParameterTemplate {
  id: string
  name: string
  category: string
  description: string
  parameters: string
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

export default api