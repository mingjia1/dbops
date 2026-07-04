/** Pure utility functions extracted from ClusterDeploy.tsx for testability */

// Re-export step types from deployStepHelper for convenience
export type { DeployStep, DeployState } from './deployStepHelper'
export { processStepEvent } from './deployStepHelper'

// ---- Constants ----

export const STAGE_ORDER = ['环境检查', '安装二进制', '配置集群', '启动节点', '集群验证'] as const
export type StageName = (typeof STAGE_ORDER)[number]

export const STEP_TYPE_CN: Record<string, string> = {
  validate: '校验',
  sync: '同步',
  bootstrap: '引导',
  join: '加入',
  deploy: '部署',
  configure: '配置',
  verify: '验证',
}

export const DEPLOY_SUBSTEPS: Record<string, string[]> = {
  '环境检查': ['检查主机连通性', '验证端口可用性', '检查磁盘空间', '检查系统依赖'],
  '安装二进制': ['下载 MySQL 安装包', '解压安装包', '创建数据目录', '设置文件权限'],
  '配置集群': ['生成 my.cnf', '配置复制用户', '配置复制参数', '写入集群拓扑'],
  '启动节点': ['启动 MySQL 服务', '等待端口就绪', '执行健康检查', '验证服务状态'],
  '集群验证': ['检查复制延迟', '验证 GTID 一致性', '执行数据校验', '生成部署报告'],
}

// ---- Types ----

export type ArchType = 'ha' | 'mha' | 'mgr' | 'pxc'

export interface DeployNodeProgress {
  instance_id?: string
  name?: string
  host?: string
  port?: number
  role?: string
  status?: string
  current_step?: string
  progress?: number
  message?: string
}

export interface DeployStepView {
  name?: string
  id?: string
  status?: string
  message?: string
  started_at?: string
  completed_at?: string
  type?: string
  target_node?: string
  depends_on?: string[]
}

export interface DeployResult {
  deployment_id: string
  cluster_id: string
  cluster_type: ArchType
  status: string
  stage?: string
  progress: number
  message: string
  mysql_user?: string
  mysql_password?: string
  started_at: string
  finished_at?: string
  nodes?: DeployNodeProgress[]
  steps?: DeployStepView[]
  logs?: string[]
}

// ---- Pure helpers ----

export const normalizeStatus = (status?: string): string => (status || '').trim().toLowerCase()

export const getStatusCategory = (status?: string): string => {
  const norm = normalizeStatus(status)
  if (['success', 'completed', 'succeeded', 'ok'].includes(norm)) return 'success'
  if (['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(norm)) return 'failed'
  if (['partial', 'partial_success'].includes(norm)) return 'partial'
  if (norm === 'destroyed') return 'destroyed'
  if (norm === 'running') return 'running'
  if (norm === 'pending') return 'pending'
  return norm || 'unknown'
}

export const isCompletedDeployStatus = (status?: string): boolean => getStatusCategory(status) === 'success'
export const isFailedDeployStatus = (status?: string): boolean => getStatusCategory(status) === 'failed'
export const isPartialDeployStatus = (status?: string): boolean => getStatusCategory(status) === 'partial'
export const isDestroyedDeployStatus = (status?: string): boolean => getStatusCategory(status) === 'destroyed'
export const isTerminalDeployStatus = (status?: string): boolean =>
  isCompletedDeployStatus(status) || isFailedDeployStatus(status) || isPartialDeployStatus(status) || isDestroyedDeployStatus(status)

export const deploymentProgress = (status?: string, progress?: number): number => {
  if (typeof progress === 'number') return progress
  return isTerminalDeployStatus(status) ? 100 : 0
}

export const deploymentProgressStatus = (status?: string): 'active' | 'exception' | 'success' | 'normal' => {
  if (isFailedDeployStatus(status)) return 'exception'
  if (isCompletedDeployStatus(status) || isDestroyedDeployStatus(status)) return 'success'
  if (isPartialDeployStatus(status)) return 'normal'
  return 'active'
}

export const deploymentStepStatus = (status?: string): 'process' | 'error' | 'finish' => {
  if (isFailedDeployStatus(status)) return 'error'
  if (isCompletedDeployStatus(status) || isDestroyedDeployStatus(status)) return 'finish'
  if (isPartialDeployStatus(status)) return 'error'
  return 'process'
}

export const clampProgress = (progress?: number): number => {
  if (typeof progress !== 'number' || Number.isNaN(progress)) return 0
  return Math.max(0, Math.min(100, Math.round(progress)))
}

export const stepStatusToAntd = (status?: string): 'wait' | 'process' | 'finish' | 'error' => {
  const norm = normalizeStatus(status)
  if (['completed', 'success', 'succeeded', 'ok', 'done'].includes(norm)) return 'finish'
  if (['running', 'processing', 'active'].includes(norm)) return 'process'
  if (['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(norm)) return 'error'
  return 'wait'
}

export const stepProgressPercent = (
  step: DeployStepView,
  idx: number,
  steps: DeployStepView[],
  overallProgress?: number,
): number => {
  const status = stepStatusToAntd(step.status)
  if (status === 'finish') return 100
  if (status === 'error') return Math.max(5, clampProgress(overallProgress))
  if (status === 'wait') return 0
  const completedBefore = steps.slice(0, idx).filter((item) => stepStatusToAntd(item.status) === 'finish').length
  const stepSpan = 100 / Math.max(steps.length, 1)
  return clampProgress(((overallProgress || 0) - completedBefore * stepSpan) / stepSpan * 100)
}

export const majorVersion = (version?: string): number => {
  const major = Number(String(version || '').split('.')[0])
  return Number.isFinite(major) ? major : 0
}

export const versionSupportsArch = (arch: ArchType, version?: string): boolean => {
  if (arch === 'mgr') return majorVersion(version) >= 8
  return true
}

export const createMgrGroupName = (): string => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  const hex = `${Date.now().toString(16)}${Math.random().toString(16).slice(2)}00000000000000000000000000000000`.slice(0, 32)
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-4${hex.slice(13, 16)}-8${hex.slice(17, 20)}-${hex.slice(20, 32)}`
}

export const parseMySQLConfig = (text?: string): Record<string, string> => {
  const config: Record<string, string> = {}
  ;(text || '').split('\n').forEach((line) => {
    const item = line.trim()
    if (!item || item.startsWith('#')) return
    const idx = item.indexOf('=')
    if (idx <= 0) return
    config[item.slice(0, idx).trim()] = item.slice(idx + 1).trim()
  })
  return config
}

export const normalizeDeployment = (data: any): DeployResult => ({
  deployment_id: data.deployment_id || data.id,
  cluster_id: data.cluster_id || data.deployment_id || data.id,
  cluster_type: data.cluster_type,
  status: data.status || 'pending',
  progress: deploymentProgress(data.status, data.progress),
  stage: data.stage,
  message: data.message || '',
  mysql_user: data.mysql_user,
  mysql_password: data.mysql_password,
  started_at: data.started_at || data.created_at,
  finished_at: data.finished_at || data.updated_at,
  nodes: Array.isArray(data.nodes) ? data.nodes : [],
  steps: Array.isArray(data.steps) ? data.steps : [],
  logs: Array.isArray(data.logs) ? data.logs : [],
})
