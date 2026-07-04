/** Shared backup helper types and pure functions extracted from BackupManage.tsx */

// ---- Types ----

export interface BackupRecord {
  id: string
  task_id: string
  instance_id: string
  backup_type: string
  status: string
  message?: string
  size: string
  file_path: string
  created_at: string
}

export interface BackupPolicy {
  id: string
  instance_id: string
  backup_type: string
  schedule: string
  retention_days: number
  storage_type: string
  storage_path: string
  enabled: boolean
  created_at: string
}

// ---- Constants ----

export const BACKUP_TYPE_LABELS: Record<string, string> = {
  full: '全量',
  incremental: '增量',
  logical: '逻辑',
}

export const BACKUP_TYPE_COLORS: Record<string, string> = {
  full: 'blue',
  incremental: 'green',
  logical: 'orange',
}

// ---- Status Checkers ----

export const isFailedBackupStatus = (status?: string): boolean => {
  const normalized = (status || '').toLowerCase()
  return ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(normalized)
}

export const isCompletedBackupStatus = (status?: string): boolean => {
  const normalized = (status || '').toLowerCase()
  return ['completed', 'success', 'succeeded', 'ok'].includes(normalized)
}

export const isActiveBackupStatus = (status?: string): boolean => {
  const normalized = (status || '').toLowerCase()
  return ['pending', 'running', 'submitted', 'accepted', 'queued'].includes(normalized)
}

// ---- Formatting ----

export const formatBackupStatus = (status: string): string => {
  if (status === 'completed') return '已完成'
  if (status === 'running') return '运行中'
  if (status === 'failed') return '失败'
  return status || '-'
}

export const formatBackupSize = (bytes?: number): string => {
  if (!bytes) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
}
