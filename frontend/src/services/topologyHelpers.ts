import type { Instance } from './api'

// ---- Types ----

export interface TopologyNode {
  id: string
  name: string
  role: string
  status: string
  cluster_id: string
}

export interface TopologyEdge {
  source_id: string
  target_id: string
  type: string
  label: string
}

export interface ClusterGraph {
  clusterId: string
  mode: string
  nodes: TopologyNode[]
  edges: TopologyEdge[]
}

// ---- Constants ----

export const primaryRoles = new Set(['master', 'primary', 'primary_master', 'bootstrap'])
export const replicaRoles = new Set(['slave', 'replica', 'secondary'])

// ---- Pure Helpers ----

export const roleColor = (role?: string): string => {
  const normalized = (role || '').toLowerCase()
  if (primaryRoles.has(normalized)) return 'blue'
  if (replicaRoles.has(normalized)) return 'green'
  if (normalized === 'manager') return 'purple'
  return 'default'
}

export const statusColor = (status?: string): string => {
  const normalized = (status || '').toLowerCase()
  if (normalized === 'healthy' || normalized === 'running' || normalized === 'success') return 'success'
  if (normalized === 'failed' || normalized === 'stopped' || normalized === 'unhealthy') return 'error'
  return 'default'
}

export const endpointOf = (item?: Instance): string =>
  `${item?.connection?.host || item?.host || '-'}:${item?.connection?.port || item?.port || '-'}`

export const relationLabel = (value?: string): string => {
  const normalized = (value || '').toLowerCase()
  if (normalized === 'pxc') return '集群同步'
  if (normalized === 'async') return '异步复制'
  if (normalized === 'semi-sync' || normalized === 'semisync') return '半同步'
  if (normalized === 'group_replication' || normalized === 'mgr') return '组复制'
  if (normalized === 'replication') return '复制'
  return value || '复制'
}

export const parseSlaveIds = (value?: string): string[] => {
  if (!value) return []
  try {
    const parsed = JSON.parse(value)
    return Array.isArray(parsed) ? parsed.filter(Boolean).map(String) : []
  } catch {
    return value.split(',').map((item) => item.trim()).filter(Boolean)
  }
}

export const inferEdgesFromInstances = (clusterInstances: Instance[]): TopologyEdge[] => {
  const ids = new Set(clusterInstances.map((item) => item.id))
  const edgeKeys = new Set<string>()
  const edges: TopologyEdge[] = []
  const addEdge = (sourceId?: string, targetId?: string, label?: string) => {
    if (!sourceId || !targetId || !ids.has(sourceId) || !ids.has(targetId)) return
    const key = `${sourceId}->${targetId}`
    if (edgeKeys.has(key)) return
    edgeKeys.add(key)
    edges.push({ source_id: sourceId, target_id: targetId, type: 'replication', label: label || 'replication' })
  }

  clusterInstances.forEach((instance) => {
    const mode = instance.topology?.replication_mode || instance.status?.replication_status || 'replication'
    addEdge(instance.topology?.master_id, instance.id, mode)
    parseSlaveIds(instance.topology?.slave_ids).forEach((slaveId) => addEdge(instance.id, slaveId, mode))
  })
  if (edges.length > 0 || clusterInstances.length <= 1) return edges

  const primary = clusterInstances.find((instance) =>
    primaryRoles.has((instance.status?.role || '').toLowerCase()),
  ) || clusterInstances[0]
  clusterInstances.forEach((instance) => {
    if (instance.id !== primary.id) addEdge(primary.id, instance.id, primary.topology?.replication_mode || 'replication')
  })
  return edges
}
