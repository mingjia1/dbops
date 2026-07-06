/** Shared status utility functions for deploy, HA, and other domains */

// ---- Normalize ----

export const normalizeStatus = (status?: string): string => (status || '').trim().toLowerCase()

// ---- Status category classification ----

export const getStatusCategory = (status?: string): string => {
  const norm = normalizeStatus(status)
  if (['success', 'completed', 'succeeded', 'ok'].includes(norm)) return 'success'
  if (['failed', 'error', 'timeout', 'cancelled', 'canceled', 'interrupted'].includes(norm)) return 'failed'
  if (['partial', 'partial_success'].includes(norm)) return 'partial'
  if (norm === 'destroyed') return 'destroyed'
  if (norm === 'skipped') return 'skipped'
  if (norm === 'running') return 'running'
  if (norm === 'pending') return 'pending'
  return norm || 'unknown'
}

// ---- Generic status checks (domain-agnostic) ----

export const isCompletedStatus = (status?: string): boolean => getStatusCategory(status) === 'success'
export const isFailedStatus = (status?: string): boolean => getStatusCategory(status) === 'failed'
export const isPartialStatus = (status?: string): boolean => getStatusCategory(status) === 'partial'
export const isDestroyedStatus = (status?: string): boolean => getStatusCategory(status) === 'destroyed'
export const isSkippedStatus = (status?: string): boolean => getStatusCategory(status) === 'skipped'
export const isTerminalStatus = (status?: string): boolean =>
  isCompletedStatus(status) || isFailedStatus(status) || isPartialStatus(status) || isDestroyedStatus(status)

// ---- Domain-specific aliases (convenience wrappers for deploy domain) ----

export const isCompletedDeployStatus = isCompletedStatus
export const isFailedDeployStatus = isFailedStatus
export const isPartialDeployStatus = isPartialStatus
export const isDestroyedDeployStatus = isDestroyedStatus
export const isTerminalDeployStatus = isTerminalStatus

// ---- Domain-specific aliases (convenience wrappers for HA domain) ----

export const isCompletedHAStatus = isCompletedStatus
export const isFailedHAStatus = isFailedStatus
export const isPartialHAStatus = isPartialStatus
export const isSkippedHAStatus = isSkippedStatus
