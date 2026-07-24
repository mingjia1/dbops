/** Pure utility functions extracted from ClusterDeploy.tsx for testability */

// Re-export step types from deployStepHelper for convenience
export type { DeployStep, DeployState } from './deployStepHelper'
export { processStepEvent } from './deployStepHelper'

// Import shared status helpers (needed as local bindings for functions below)
import {
  normalizeStatus,
  getStatusCategory,
  isCompletedDeployStatus,
  isFailedDeployStatus,
  isPartialDeployStatus,
  isDestroyedDeployStatus,
  isTerminalDeployStatus,
} from './statusHelpers'

// Re-export for consumers
export {
  normalizeStatus,
  getStatusCategory,
  isCompletedDeployStatus,
  isFailedDeployStatus,
  isPartialDeployStatus,
  isDestroyedDeployStatus,
  isTerminalDeployStatus,
}

// ---- Constants ----

export const STAGE_ORDER = ['环境检查', '安装二进制', '配置集群', '启动节点', '集群验证'] as const
export type StageName = (typeof STAGE_ORDER)[number]

export function stageIndexFromProgress(progress?: number): number {
  const p = Math.max(0, Math.min(100, Number(progress || 0)))
  if (p >= 95) return 4
  if (p >= 75) return 3
  if (p >= 50) return 2
  if (p >= 20) return 1
  return 0
}

export function currentStageIndex(stage?: string, progress?: number): number {
  const idx = stage ? STAGE_ORDER.indexOf(stage as StageName) : -1
  return idx >= 0 ? idx : stageIndexFromProgress(progress)
}

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

// ---- Status-dependent helpers (deploy-specific logic on top of shared status checks) ----

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
  if (['failed', 'error', 'timeout', 'cancelled', 'canceled', 'interrupted'].includes(norm)) return 'error'
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

export const deploymentPayloadFingerprint = (payload: any): string => stableStringify({
  cluster_id: payload?.cluster_id,
  cluster_type: payload?.cluster_type,
  mysql_version: payload?.mysql?.version,
  nodes: (payload?.nodes || []).map((node: any) => ({
    instance_id: node.instance_id,
    host_id: node.host_id,
    host: node.host,
    role: node.role,
    mysql_port: node.mysql_port,
    data_dir: node.data_dir,
    basedir: node.basedir,
    server_id: node.server_id,
    custom: node.custom,
  })),
  middleware: payload?.custom?.middleware,
  tools: payload?.custom?.tools,
})

const stableStringify = (value: any): string => {
  if (Array.isArray(value)) return `[${value.map((item) => stableStringify(item)).join(',')}]`
  if (value && typeof value === 'object') {
    return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${stableStringify(value[key])}`).join(',')}}`
  }
  return JSON.stringify(value)
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

export function applyRelayConfig(payload: Record<string, any>) {
  const result: Record<string, any> = { ...payload, custom: { ...(payload.custom || {}) } }
  const custom = result.custom

  // Auto-inject relay_url from system settings if configured
  try {
    const relayCfg = localStorage.getItem('dbops_relay_server')
    if (relayCfg) {
      const parsed = JSON.parse(relayCfg)
      const platformIp = typeof window !== 'undefined' ? window.location.hostname || '10.3.67.52' : '10.3.67.52'
      const defaultVersion = parsed.default_version || payload?.mysql?.version || '8.0.36'
      const resolveVars = (url: string) => {
        const parts = defaultVersion.split('.')
        const majorMinor = parts.length >= 2 ? `${parts[0]}.${parts[1]}` : defaultVersion
        return url
          .replace(/\$\{platform_ip\}/g, platformIp)
          .replace(/\$\{version\}/g, defaultVersion)
          .replace(/\$\{major_minor\}/g, majorMinor)
          .replace(/\$\{major\}/g, parts[0] || defaultVersion)
          .replace(/\$\{minor\}/g, parts[1] || '')
      }
      // Build relay_url from sources (new format) or legacy relay_url (old format)
      let relayUrl = ''
      if (parsed.sources && parsed.sources.length > 0) {
        const firstEnabled = parsed.sources.find((s: any) => s.enabled && s.url)
        if (firstEnabled) relayUrl = resolveVars(firstEnabled.url).replace(/\/+$/, '')
      } else if (parsed.relay_url) {
        relayUrl = resolveVars(parsed.relay_url).replace(/\/+$/, '')
      }
      if (relayUrl && parsed.relay_path) {
        relayUrl += '/' + parsed.relay_path.replace(/^\/+/, '').replace(/\/+$/, '')
      }
      if (relayUrl) custom.relay_url = relayUrl
      if (typeof window !== 'undefined') custom.relay_upload_url = window.location.origin + '/api/v1/relay/upload'
    }
  } catch { /* ignore */ }

  // Propagate relay_url to each node for precheck repair
  if (custom.relay_url && Array.isArray(payload.nodes)) {
    result.nodes = payload.nodes.map((node: any) => {
      if (!node.relay_url) return { ...node, relay_url: custom.relay_url }
      return node
    })
  }

  return result
}

/**
 * Build deployment payload from form values.
 * Extracted from ClusterDeploy.tsx for testability.
 */
export const buildDeployPayload = (
  arch: ArchType,
  values: any,
  credential: { username: string; password: string },
): Record<string, any> => {
  const replicaHostIDs = values.replica_host_ids || (values.replica_host_id ? [values.replica_host_id] : [])
  const nodes: any[] = []
  const addNode = (hostID: string, role: string, port?: number, extra?: Record<string, any>) => {
    if (!hostID) return
    nodes.push({
      host_id: hostID,
      role,
      mysql_port: port || values.mysql_port || 3306,
      data_dir: extra?.data_dir,
      basedir: extra?.basedir,
      server_id: extra?.server_id,
      custom: extra?.custom,
      package_url: extra?.package_url,
      relay_url: extra?.relay_url,
    })
  }

  if (arch === 'ha') {
    addNode(values.master_host_id, 'master', values.mysql_port, { data_dir: values.master_data_dir, server_id: values.master_server_id })
    replicaHostIDs.forEach((hostID: string, index: number) => addNode(hostID, 'replica', values.replica_port || values.mysql_port || 3306, {
      data_dir: values.replica_data_dir,
      server_id: values.replica_server_id || (values.master_server_id ? Number(values.master_server_id) + index + 1 : undefined),
    }))
  } else if (arch === 'mha') {
    addNode(values.manager_host_id, 'manager', values.manager_port || values.mysql_port)
    addNode(values.master_host_id, 'master', values.mysql_port)
    replicaHostIDs.forEach((hostID: string) => addNode(hostID, 'replica', values.replica_port || values.mysql_port || 3306))
  } else if (arch === 'mgr') {
    addNode(values.master_host_id, 'primary', values.mysql_port, { server_id: values.master_server_id, custom: { local_port: values.local_port } })
    replicaHostIDs.forEach((hostID: string, index: number) => addNode(hostID, 'secondary', values.replica_port || values.mysql_port || 3306, {
      data_dir: values.replica_data_dir,
      server_id: values.replica_server_id || (values.master_server_id ? Number(values.master_server_id) + index + 1 : undefined),
      custom: values.local_port ? { local_port: Number(values.local_port) + index + 1 } : undefined,
    }))
  } else {
    addNode(values.master_host_id, 'bootstrap', values.mysql_port, { data_dir: values.master_data_dir })
    replicaHostIDs.forEach((hostID: string) => addNode(hostID, 'secondary', values.replica_port || values.mysql_port || 3306, { data_dir: values.replica_data_dir }))
  }

  const custom: Record<string, any> = {}
  // Auto-inject relay_url from system settings if configured
  try {
    const relayCfg = localStorage.getItem('dbops_relay_server')
    if (relayCfg) {
      const parsed = JSON.parse(relayCfg)
      const platformIp = window.location.hostname || '10.3.67.52'
      const defaultVersion = parsed.default_version || values.mysql_version || '8.0.36'
      const resolveVars = (url: string) => {
        const parts = defaultVersion.split('.')
        const majorMinor = parts.length >= 2 ? `${parts[0]}.${parts[1]}` : defaultVersion
        return url
          .replace(/\$\{platform_ip\}/g, platformIp)
          .replace(/\$\{version\}/g, defaultVersion)
          .replace(/\$\{major_minor\}/g, majorMinor)
          .replace(/\$\{major\}/g, parts[0] || defaultVersion)
          .replace(/\$\{minor\}/g, parts[1] || '')
      }
      // Build relay_url from sources (new format) or legacy relay_url (old format)
      let relayUrl = ''
      if (parsed.sources && parsed.sources.length > 0) {
        const firstEnabled = parsed.sources.find((s: any) => s.enabled && s.url)
        if (firstEnabled) relayUrl = resolveVars(firstEnabled.url).replace(/\/+$/, '')
      } else if (parsed.relay_url) {
        relayUrl = resolveVars(parsed.relay_url).replace(/\/+$/, '')
      }
      if (relayUrl && parsed.relay_path) {
        relayUrl += '/' + parsed.relay_path.replace(/^\/+/, '').replace(/\/+$/, '')
      }
      if (relayUrl) custom.relay_url = relayUrl
      custom.relay_upload_url = window.location.origin + '/api/v1/relay/upload'
    }
  } catch { /* ignore */ }

  // Propagate relay_url to each node for precheck repair
  if (custom.relay_url) {
    for (const node of nodes) {
      if (!node.relay_url) node.relay_url = custom.relay_url
    }
  }

  if (arch === 'ha' && values.semi_sync_enabled !== undefined) custom.semi_sync_enabled = !!values.semi_sync_enabled
  if (arch === 'mha') {
    if (values.vip) custom.vip = values.vip
    if (values.vip_interface) custom.vip_interface = values.vip_interface
    if (values.ping_interval) custom.ping_interval = values.ping_interval
    if (values.ping_retry) custom.ping_retry = values.ping_retry
    if (values.ssh_user) custom.ssh_user = values.ssh_user
  }
  if (arch === 'mgr') {
    if (values.group_name) custom.group_name = values.group_name
    if (values.local_port) custom.local_port = values.local_port
  }
  if (arch === 'pxc') {
    if (values.wsrep_port) custom.wsrep_port = values.wsrep_port
    if (values.cluster_name) custom.cluster_name = values.cluster_name
    if (values.sst_method) custom.sst_method = values.sst_method
    if (values.wsrep_sst_port) custom.wsrep_sst_port = values.wsrep_sst_port
    if (values.wsrep_ssl_enabled !== undefined) custom.wsrep_ssl_enabled = !!values.wsrep_ssl_enabled
  }

  return {
    cluster_id: values.cluster_id,
    name: values.cluster_id,
    cluster_type: arch,
    mode: 'real',
    mysql: {
      version: values.mysql_version || '8.0',
      user: credential.username,
      password: credential.password,
      package_url: values.package_url,
      package_checksum: values.package_checksum,
      config: parseMySQLConfig(values.mysql_config_text),
    },
    replication: {
      user: values.repl_user,
      password: values.repl_password,
      mode: arch === 'mgr' ? 'single-primary' : arch === 'pxc' ? 'galera' : 'async',
    },
    nodes,
    custom,
  }
}
