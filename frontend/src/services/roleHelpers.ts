/** Shared role/normalize helper functions for cluster topology & HA domains */

export const normalizeRole = (role?: string): string => (role || '').toLowerCase()

export const isPrimaryRole = (role?: string): boolean =>
  ['master', 'primary', 'primary_master', 'bootstrap'].includes(normalizeRole(role))

export const isReplicaRole = (role?: string): boolean =>
  ['slave', 'secondary', 'replica', 'donor', 'joiner'].includes(normalizeRole(role))
