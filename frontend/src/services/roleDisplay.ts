export type ClusterArch = 'ha' | 'mha' | 'mgr' | 'pxc' | string | undefined

export const formatClusterRole = (arch: ClusterArch, role?: string) => {
  const normalizedArch = (arch || '').toLowerCase()
  const normalizedRole = (role || '').toLowerCase()
  if (!normalizedRole) return '-'

  if (normalizedArch === 'ha' || normalizedArch === 'mha' || normalizedArch === 'replication') {
    if (normalizedRole === 'replica' || normalizedRole === 'secondary') return 'slave'
    if (normalizedRole === 'primary' || normalizedRole === 'bootstrap') return 'master'
  }

  if (normalizedArch === 'mgr') {
    if (normalizedRole === 'master' || normalizedRole === 'bootstrap') return 'primary'
    if (normalizedRole === 'replica' || normalizedRole === 'slave') return 'secondary'
  }

  if (normalizedArch === 'pxc') {
    if (normalizedRole === 'master' || normalizedRole === 'primary') return 'bootstrap'
    if (normalizedRole === 'replica' || normalizedRole === 'slave') return 'secondary'
  }

  return role || '-'
}

export const inferArchFromReplicationMode = (value?: string) => {
  const normalized = (value || '').toLowerCase()
  if (normalized === 'mgr' || normalized.includes('group_replication')) return 'mgr'
  if (normalized === 'pxc' || normalized.includes('galera')) return 'pxc'
  if (normalized === 'mha') return 'mha'
  if (normalized === 'replication' || normalized === 'async' || normalized === 'semi-sync' || normalized === 'semisync') return 'ha'
  return normalized
}
