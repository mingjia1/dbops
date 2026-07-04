import type { Instance } from './api'

// ---- Types ----

export interface HAClusterStatus {
  cluster_id: string
  master: any
  slaves: any[]
  history: any[]
}

export interface PreflightResult {
  cluster_id: string
  target_master_id?: string
  current_master_id: string
  current_master_healthy: boolean
  healthy_slave_count: number
  slave_count: number
  max_replication_lag: number
  gtid_consistent: boolean
  topology_consistent: boolean
  real_primary_id?: string
  platform_primary_id?: string
  pass: boolean
  blocking_reasons?: string[]
  warnings?: string[]
}

// ---- Node/Instance Helpers ----

export const haNodeID = (node: any): string => node?.instance_id || node?.id || '-'

export const haNodeEndpoint = (node: any): string => {
  if (!node) return '-'
  if (node.host && node.port) return `${node.host}:${node.port}`
  return node.host || '-'
}

export const instanceEndpoint = (instance?: Instance): string => {
  if (!instance) return '-'
  const host = instance.host || instance.connection?.host
  const port = instance.port || instance.connection?.port
  if (host && port) return `${host}:${port}`
  return host || '-'
}

// ---- Role/Normalize Helpers ----

export const normalizeRole = (role?: string): string => (role || '').toLowerCase()

export const isPrimaryRole = (role?: string): boolean =>
  ['master', 'primary', 'primary_master', 'bootstrap'].includes(normalizeRole(role))

export const isReplicaRole = (role?: string): boolean =>
  ['slave', 'secondary', 'replica', 'donor', 'joiner'].includes(normalizeRole(role))

// ---- Status Helpers ----

export const isFailedHAStatus = (status?: string): boolean =>
  ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes((status || '').toLowerCase())

export const isCompletedHAStatus = (status?: string): boolean =>
  ['completed', 'success', 'succeeded', 'ok'].includes((status || '').toLowerCase())

export const isSkippedHAStatus = (status?: string): boolean =>
  (status || '').toLowerCase() === 'skipped'

export const isPartialHAStatus = (status?: string): boolean =>
  (status || '').toLowerCase() === 'partial_success'

// ---- Cluster Architecture Detection ----

export const isMGRInstance = (instance: Instance): boolean => {
  const mode = (instance.topology?.replication_mode || '').toLowerCase()
  const repl = (instance.status?.replication_status || '').toLowerCase()
  return mode === 'mgr' || repl === 'mgr' || mode.includes('group_replication') || repl.includes('group_replication')
}

export const isPXCInstance = (instance: Instance): boolean => {
  const mode = (instance.topology?.replication_mode || '').toLowerCase()
  const repl = (instance.status?.replication_status || '').toLowerCase()
  return mode === 'pxc' || repl === 'pxc' || mode.includes('galera') || repl.includes('galera') || mode.includes('wsrep') || repl.includes('wsrep')
}

export const detectClusterArch = (instances: Instance[]): 'ha' | 'mha' | 'mgr' | 'pxc' | '' => {
  if (instances.some(isMGRInstance)) return 'mgr'
  if (instances.some(isPXCInstance)) return 'pxc'
  const mode = (instances[0]?.topology?.replication_mode || '').toLowerCase()
  if (mode === 'mha') return 'mha'
  if (instances.length > 0) return 'ha'
  return ''
}
