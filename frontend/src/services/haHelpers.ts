import type { Instance } from './api'
import {
  isCompletedHAStatus,
  isFailedHAStatus,
  isPartialHAStatus,
  isSkippedHAStatus,
} from './statusHelpers'
import { normalizeRole, isPrimaryRole, isReplicaRole } from './roleHelpers'

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

// ---- Status Helpers (re-exported from shared statusHelpers) ----
export { isCompletedHAStatus, isFailedHAStatus, isPartialHAStatus, isSkippedHAStatus } from './statusHelpers'

// ---- Role/Normalize Helpers (re-exported from shared roleHelpers) ----
export { normalizeRole, isPrimaryRole, isReplicaRole } from './roleHelpers'

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
