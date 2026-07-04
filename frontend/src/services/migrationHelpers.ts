/** Shared migration helper functions extracted from MigrationManage.tsx */

// ---- Types ----

export interface MigrationTask {
  id: string
  migration_type?: 'physical' | 'replication' | 'gtid'
  strategy?: 'physical' | 'replication' | 'gtid'
  source_instance?: string
  target_instance?: string
  source_instance_id?: string
  target_instance_id?: string
  status: 'pending' | 'preparing' | 'migrating' | 'running' | 'verifying' | 'switching' | 'completed' | 'failed' | 'cancelled'
  progress: number
  started_at: string
  completed_at?: string
  error?: string
  error_message?: string
  steps?: Array<{ name: string; status: string; message?: string; started_at?: string; completed_at?: string }>
  logs?: string[]
}

export interface MigrationProgressStep {
  stage: string
  progress: number
  details: string
}

export interface MigrationProgressResponse {
  task_id: string
  status: string
  progress: number
  current_step?: string
  total_steps?: number
  completed_steps?: number
  data_transferred?: number
  estimated_time?: number
  updated_at?: string
  steps?: Array<{ name: string; status: string; message?: string; started_at?: string; completed_at?: string }>
  logs?: string[]
}

// ---- Constants ----

export const MIGRATION_SUBSTEPS: Record<string, string[]> = {
  '数据导出': ['锁定源表', '执行 mysqldump/xtrabackup', '生成校验和', '记录导出位置'],
  '数据传输': ['建立目标连接', '传输数据文件', '验证传输完整性'],
  '数据导入': ['准备目标实例', '导入数据文件', '重建索引', '更新系统表'],
  '一致性校验': ['表行数对比', 'CRC32 校验', 'GTID 一致性检查'],
  '切换': ['停止源实例写入', '等待目标追上', '切换业务连接', '验证新主可用'],
}

export const activeMigrationStatuses = new Set(['pending', 'preparing', 'migrating', 'running', 'verifying', 'switching'])

// ---- Status Checkers ----

export const isActiveMigrationStatus = (status?: string): boolean =>
  activeMigrationStatuses.has((status || '').toLowerCase())

export const isFailedMigrationStatus = (status?: string): boolean => {
  const normalized = (status || '').toLowerCase()
  return ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(normalized)
}

export const isCompletedMigrationStatus = (status?: string): boolean => {
  const normalized = (status || '').toLowerCase()
  return ['completed', 'success', 'succeeded', 'ok'].includes(normalized)
}

export const migrationProgressStatus = (status?: string): 'success' | 'exception' | 'active' | 'normal' => {
  if (isCompletedMigrationStatus(status)) return 'success'
  if (isFailedMigrationStatus(status)) return 'exception'
  if (isActiveMigrationStatus(status)) return 'active'
  return 'normal'
}

export const migrationStatusColor = (status?: string): string => {
  const normalized = (status || '').toLowerCase()
  if (isCompletedMigrationStatus(normalized)) return 'success'
  if (isFailedMigrationStatus(normalized)) return 'error'
  if (normalized === 'verifying' || normalized === 'switching') return 'warning'
  if (isActiveMigrationStatus(normalized)) return 'processing'
  return 'default'
}

// ---- Helpers ----

export const formatBytes = (value?: number): string => {
  if (!value || value <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = value
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index += 1
  }
  return `${size.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

export const buildProgressDetails = (progress: MigrationProgressResponse): MigrationProgressStep[] => {
  const totalSteps = progress.total_steps || 0
  const completedSteps = progress.completed_steps || 0
  const stepPercent = totalSteps > 0 ? Math.round((completedSteps / totalSteps) * 100) : progress.progress || 0
  return [
    {
      stage: progress.current_step || '迁移执行',
      progress: progress.progress || 0,
      details: `状态: ${progress.status || 'unknown'}`,
    },
    {
      stage: '阶段完成度',
      progress: stepPercent,
      details: totalSteps > 0 ? `${completedSteps}/${totalSteps} 个阶段已完成` : '等待后端返回阶段数量',
    },
    {
      stage: '数据传输',
      progress: progress.progress || 0,
      details: `已传输 ${formatBytes(progress.data_transferred)}${progress.estimated_time ? `, 预计剩余 ${progress.estimated_time}s` : ''}`,
    },
  ]
}

export const buildCreatePayload = (values: any, strategy: 'physical' | 'replication' | 'gtid') => ({
  name: `${strategy}-${Date.now()}`,
  source_instance_id: values.source_instance,
  target_instance_id: values.target_instance,
  strategy,
  config: JSON.stringify(values),
})

export const taskFromResult = (values: any, strategy: 'physical' | 'replication' | 'gtid', res: any): MigrationTask | null => {
  const taskId = res?.data?.task_id || res?.data?.id
  if (!taskId) return null
  return {
    id: taskId,
    migration_type: strategy,
    strategy,
    source_instance: values.source_instance,
    target_instance: values.target_instance,
    source_instance_id: values.source_instance,
    target_instance_id: values.target_instance,
    status: res?.data?.status || 'migrating',
    progress: typeof res?.data?.progress === 'number' ? res.data.progress : 0,
    started_at: res?.data?.started_at || new Date().toISOString(),
  }
}

export const getCurrentSubstages = (status: string): string[] => {
  if (status === 'verifying') return MIGRATION_SUBSTEPS['一致性校验']
  if (status === 'switching') return MIGRATION_SUBSTEPS['切换']
  return MIGRATION_SUBSTEPS['数据导出']
}
