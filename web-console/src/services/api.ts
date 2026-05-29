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

export default api