/** Shared upgrade helper functions extracted from UpgradeManage.tsx */

// ---- Constants ----

export const UPGRADE_STAGES = ['预检查', '备份数据', '停止服务', '安装新版本', '数据升级', '启动服务', '验证']
export const ROLLING_UPGRADE_STAGES = ['集群检查', '升级从库', '从库验证', '角色切换', '升级原主库', '重建拓扑', '最终验证']

export const UPGRADE_SUBSTEPS: Record<string, string[]> = {
  '预检查': ['检查磁盘空间', '验证版本兼容性', '检查实例状态', '验证备份状态'],
  '备份数据': ['执行全量备份', '验证备份完整性', '记录备份路径'],
  '停止服务': ['通知应用断开连接', '等待事务完成', '停止 MySQL 服务'],
  '安装新版本': ['下载安装包', '解压新版本', '替换二进制文件', '验证安装路径'],
  '数据升级': ['执行 mysql_upgrade', '应用系统表变更', '验证数据字典'],
  '启动服务': ['启动 MySQL 服务', '等待端口就绪', '执行健康检查'],
  '验证': ['连接测试', '查询系统变量', '执行数据校验', '生成升级报告'],
}

export const strategyOptions = [
  { value: 'inplace', label: '原地升级' },
  { value: 'logical', label: '逻辑迁移' },
  { value: 'rolling', label: '滚动升级' },
]

export const activeUpgradeStatuses = new Set(['pending', 'running', 'queued', 'executing'])
export const terminalUpgradeStatuses = new Set(['success', 'completed', 'failed', 'error', 'cancelled', 'canceled', 'timeout'])

// ---- Types ----

export interface UpgradeStep {
  name: string
  status: string
  message?: string
  started_at?: string
  completed_at?: string
}

export interface ActiveUpgrade {
  task_id: string
  instance_id: string
  cluster_id?: string
  strategy?: string
  task_type?: string
  status: string
  progress: number
  stage?: string
  message?: string
  steps?: UpgradeStep[]
  logs?: string[]
  started_at: string
  finished_at?: string
}

export interface UpgradeHistory {
  id: string
  instance_id: string
  instance_name?: string
  strategy?: string
  upgrade_type?: string
  task_type?: string
  plan_id?: string
  source_version?: string
  target_version?: string
  status: string
  progress?: number
  stage?: string
  message?: string
  start_time?: string
  created_at?: string
}

// ---- Status Checkers ----

export const isCompletedUpgradeStatus = (status?: string): boolean =>
  ['success', 'completed'].includes((status || '').toLowerCase())

export const isFailedUpgradeStatus = (status?: string): boolean =>
  ['failed', 'error', 'cancelled', 'canceled', 'timeout'].includes((status || '').toLowerCase())

// ---- Helper Functions ----

export const clampProgress = (progress?: number): number => {
  if (typeof progress !== 'number' || Number.isNaN(progress)) return 0
  return Math.max(0, Math.min(100, progress))
}

export const upgradeStagesFor = (upgrade?: { strategy?: string; upgrade_type?: string; task_type?: string }): string[] => {
  const rawType = `${upgrade?.strategy || ''} ${upgrade?.upgrade_type || ''} ${upgrade?.task_type || ''}`.toLowerCase()
  return rawType.includes('rolling') ? ROLLING_UPGRADE_STAGES : UPGRADE_STAGES
}

export const inferStepIndex = (progress: number, stages: string[], status?: string, stage?: string): number => {
  const normalized = (status || '').toLowerCase()
  if (isCompletedUpgradeStatus(normalized)) return stages.length - 1
  if (isFailedUpgradeStatus(normalized)) return Math.min(stages.length - 1, Math.floor((clampProgress(progress) / 100) * stages.length))
  if (stage) {
    const idx = stages.indexOf(stage)
    if (idx >= 0) return idx
  }
  return Math.min(stages.length - 1, Math.floor((clampProgress(progress) / 100) * stages.length))
}

export const buildUpgradeSteps = (upgrade: ActiveUpgrade): UpgradeStep[] => {
  if (upgrade.steps?.length) return upgrade.steps
  const stages = upgradeStagesFor(upgrade)
  const current = inferStepIndex(upgrade.progress, stages, upgrade.status, upgrade.stage)
  return stages.map((name, idx) => {
    let status: 'wait' | 'completed' | 'running' | 'failed' = 'wait'
    if (idx < current || isCompletedUpgradeStatus(upgrade.status)) status = 'completed'
    else if (idx === current && !terminalUpgradeStatuses.has((upgrade.status || '').toLowerCase())) status = 'running'
    else if (idx === current && isFailedUpgradeStatus(upgrade.status)) status = 'failed'
    return { name, status, message: idx === current ? (upgrade.message || '正在执行') : undefined }
  })
}

export const currentUpgradeStage = (upgrade: ActiveUpgrade): string => {
  const stages = upgradeStagesFor(upgrade)
  return stages[inferStepIndex(upgrade.progress, stages, upgrade.status, upgrade.stage)] || stages[0]
}

export const getUpgradeSubsteps = (planResult: any, currentStage: string): string[] => {
  if (planResult?.upgrade_path?.length) {
    return planResult.upgrade_path
      .sort((a: any, b: any) => a.order - b.order)
      .map((s: any) => s.name)
  }
  return UPGRADE_SUBSTEPS[currentStage] || UPGRADE_SUBSTEPS['预检查']
}
