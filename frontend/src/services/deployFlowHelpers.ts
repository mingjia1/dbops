import type { ArchType, DeployResult, DeployStepView } from './deployHelpers'
import { applyRelayConfig, createMgrGroupName } from './deployHelpers'

export type FlowNodeKind = 'database' | 'middleware' | 'tool'

export interface DeployFlowDbNode {
  id: string
  kind: 'database'
  role: string
  host_id?: string
  mysql_port?: number
  data_dir?: string
  basedir?: string
  server_id?: number
  custom?: Record<string, unknown>
}

export interface DeployFlowMiddlewareNode {
  id: 'keepalived' | 'proxysql'
  kind: 'middleware'
  enabled: boolean
  vip?: string
  vip_interface?: string
  proxy_host_id?: string
  proxy_host?: string
  proxy_port?: number
  agent_port?: number
  proxy_agent_port?: number
}

export interface DeployFlowToolNode {
  id: 'precheck' | 'health_check' | 'baseline_backup'
  kind: 'tool'
  enabled: boolean
  backup_type?: string
}

export type DeployFlowNode = DeployFlowDbNode | DeployFlowMiddlewareNode | DeployFlowToolNode

export interface DeployFlowSpec {
  arch: ArchType
  cluster_id: string
  mysql_version: string
  mysql_port: number
  repl_user: string
  repl_password: string
  nodes: DeployFlowDbNode[]
  middleware: DeployFlowMiddlewareNode[]
  tools: DeployFlowToolNode[]
  mysql_config_text?: string
  package_url?: string
  package_checksum?: string
  custom?: Record<string, unknown>
}

export interface FlowCredential {
  username: string
  password: string
}

export interface FlowElement {
  id: string
  source?: string
  target?: string
  position?: { x: number; y: number }
  data?: Record<string, unknown>
  style?: Record<string, unknown>
  animated?: boolean
}

export interface DeployFlowSnapshot {
  arch: ArchType
  nodes: Array<{
    id: string
    type: 'architecture' | FlowNodeKind
    data: Record<string, unknown>
  }>
  edges: Array<{ id: string; source: string; target: string }>
}

const defaultPorts: Record<ArchType, number> = {
  ha: 3306,
  mha: 3306,
  mgr: 3306,
  pxc: 3306,
}

export const createDefaultFlowSpec = (arch: ArchType): DeployFlowSpec => {
  const base = {
    arch,
    cluster_id: `${arch}-cluster-01`,
    mysql_version: '8.0',
    mysql_port: defaultPorts[arch],
    repl_user: 'repl',
    repl_password: 'Repl#2024',
    middleware: [
      { id: 'keepalived', kind: 'middleware', enabled: false, vip_interface: 'eth0' },
      { id: 'proxysql', kind: 'middleware', enabled: false, proxy_port: 6033, proxy_agent_port: 9090 },
    ] as DeployFlowMiddlewareNode[],
    tools: [
      { id: 'precheck', kind: 'tool', enabled: true },
      // 默认关闭：后置健康检查失败会使部署标记 partial，用户按需开启。
      { id: 'health_check', kind: 'tool', enabled: false },
      { id: 'baseline_backup', kind: 'tool', enabled: false, backup_type: 'full' },
    ] as DeployFlowToolNode[],
  }

  if (arch === 'ha') {
    return { ...base, nodes: [dbNode('master', 'master'), dbNode('replica-1', 'replica')] }
  }
  if (arch === 'mha') {
    return { ...base, nodes: [dbNode('manager', 'manager'), dbNode('master', 'master'), dbNode('replica-1', 'replica')] }
  }
  if (arch === 'mgr') {
    return {
      ...base,
      custom: { group_name: createMgrGroupName() },
      nodes: [
        dbNode('primary', 'primary', { custom: { local_port: 33061 } }),
        dbNode('secondary-1', 'secondary', { custom: { local_port: 33062 } }),
        dbNode('secondary-2', 'secondary', { custom: { local_port: 33063 } }),
      ],
    }
  }
  return {
    ...base,
    custom: { cluster_name: 'pxc-cluster-01', sst_method: 'xtrabackup-v2', wsrep_port: 4567 },
    nodes: [dbNode('bootstrap', 'bootstrap'), dbNode('secondary-1', 'secondary'), dbNode('secondary-2', 'secondary')],
  }
}

const dbNode = (id: string, role: string, extra: Partial<DeployFlowDbNode> = {}): DeployFlowDbNode => ({
  id,
  kind: 'database',
  role,
  mysql_port: 3306,
  ...extra,
})

export const validateFlowSpec = (spec: DeployFlowSpec): string[] => {
  const errors: string[] = []
  if (!spec.cluster_id?.trim()) errors.push('cluster_id is required')
  if (!spec.mysql_version?.trim()) errors.push('mysql_version is required')
  if (!spec.nodes.length) errors.push('at least one database node is required')

  spec.nodes.forEach((node) => {
    if (!node.host_id) errors.push(`${roleLabel(node.role)} node must select host`)
  })

  const endpointKeys = new Set<string>()
  spec.nodes
    .filter((node) => node.role !== 'manager')
    .forEach((node) => {
      const key = `${node.host_id || node.id}:${node.mysql_port || spec.mysql_port || 3306}`
      if (endpointKeys.has(key)) errors.push(`duplicate node port: ${key}`)
      endpointKeys.add(key)
    })

  if ((spec.arch === 'ha' || spec.arch === 'mha') && enabledMiddleware(spec, 'keepalived')) {
    const keepalived = middlewareByID(spec, 'keepalived')
    if (!keepalived?.vip) errors.push('Keepalived VIP is required')
  }

  if ((spec.arch === 'mgr' || spec.arch === 'pxc') && enabledMiddleware(spec, 'keepalived')) {
    errors.push('MGR/PXC do not support Keepalived in this version')
  }

  if (enabledMiddleware(spec, 'proxysql')) {
    const proxysql = middlewareByID(spec, 'proxysql')
    if (!proxysql?.proxy_host_id && !proxysql?.proxy_host) errors.push('ProxySQL host is required')
  }

  if (spec.arch === 'mgr' && spec.nodes.length < 3) errors.push('MGR requires at least 3 nodes')
  if (spec.arch === 'mha' && !spec.nodes.some((node) => node.role === 'manager')) errors.push('MHA requires a manager node')

  return Array.from(new Set(errors))
}

export const flowSpecToDeployPayload = (spec: DeployFlowSpec, credential: FlowCredential): Record<string, any> => {
  const errors = validateFlowSpec(spec)
  if (errors.length > 0) {
    throw new Error(errors.join('; '))
  }

  const custom = architectureCustom(spec)
  const middleware = {
    keepalived: keepalivedPayload(spec),
    proxysql: proxysqlPayload(spec),
  }
  const tools = {
    precheck: toolPayload(spec, 'precheck'),
    health_check: toolPayload(spec, 'health_check'),
    baseline_backup: toolPayload(spec, 'baseline_backup'),
  }

  return applyRelayConfig({
    cluster_id: spec.cluster_id,
    name: spec.cluster_id,
    cluster_type: spec.arch,
    mode: 'real',
    mysql: {
      version: spec.mysql_version,
      user: credential.username || 'root',
      password: credential.password,
      package_url: spec.package_url,
      package_checksum: spec.package_checksum,
      config: parseMySQLConfig(spec.mysql_config_text),
    },
    replication: {
      user: spec.repl_user,
      password: spec.repl_password,
      mode: spec.arch === 'mgr' ? 'single-primary' : spec.arch === 'pxc' ? 'galera' : 'async',
    },
    nodes: spec.nodes.map((node) => ({
      instance_id: node.id,
      host_id: node.host_id,
      role: node.role,
      mysql_port: node.mysql_port || spec.mysql_port || 3306,
      data_dir: node.data_dir,
      basedir: node.basedir,
      server_id: node.server_id,
      custom: node.custom,
    })),
    custom: {
      ...custom,
      flow_spec: flowSpecToGraphSnapshot(spec),
      middleware,
      tools,
    },
  })
}

export const flowSpecToGraphSnapshot = (spec: DeployFlowSpec): DeployFlowSnapshot => {
  const nodes: DeployFlowSnapshot['nodes'] = [
    {
      id: `arch-${spec.arch}`,
      type: 'architecture',
      data: { arch: spec.arch, label: spec.arch.toUpperCase() },
    },
    ...spec.nodes.map((node) => ({
      id: node.id,
      type: 'database' as const,
      data: {
        role: node.role,
        host_id: node.host_id,
        mysql_port: node.mysql_port || spec.mysql_port || 3306,
        data_dir: node.data_dir,
        basedir: node.basedir,
        server_id: node.server_id,
        custom: node.custom,
      },
    })),
    ...spec.middleware.map((node) => ({
      id: node.id,
      type: 'middleware' as const,
      data: { ...node },
    })),
    ...spec.tools.map((node) => ({
      id: node.id,
      type: 'tool' as const,
      data: { ...node },
    })),
  ]
  const edges: DeployFlowSnapshot['edges'] = []
  let lastID = `arch-${spec.arch}`
  const addEdge = (target: string) => {
    edges.push({ id: `${lastID}->${target}`, source: lastID, target })
    lastID = target
  }

  if (toolByID(spec, 'precheck')?.enabled) addEdge('precheck')
  spec.nodes.forEach((node) => addEdge(node.id))
  spec.middleware.filter((node) => node.enabled).forEach((node) => addEdge(node.id))
  spec.tools.filter((tool) => tool.id !== 'precheck' && tool.enabled).forEach((tool) => addEdge(tool.id))

  return { arch: spec.arch, nodes, edges }
}

export const deployPlanToFlowElements = (plan: any, deployment?: Partial<DeployResult>): { nodes: FlowElement[]; edges: FlowElement[] } => {
  const plannedSteps: DeployStepView[] = Array.isArray(plan?.steps) ? plan.steps : []
  const liveSteps: DeployStepView[] = Array.isArray(deployment?.steps) && deployment?.steps?.length ? deployment.steps : plannedSteps
  const stepStatus = new Map<string, DeployStepView>()
  liveSteps.forEach((step) => {
    if (step.id) stepStatus.set(step.id, step)
    if (step.name) stepStatus.set(step.name, step)
  })

  const nodes = plannedSteps.map((step: any, index: number) => {
    const live = stepStatus.get(step.id) || stepStatus.get(step.name)
    const status = live?.status || step.status || 'planned'
    return {
      id: step.id || step.name || `step-${index}`,
      position: { x: (index % 4) * 240, y: Math.floor(index / 4) * 130 },
      data: {
        label: step.name || step.id,
        status,
        type: step.type,
        target_node: step.target_node,
        message: live?.message || step.message,
      },
      style: statusStyle(status),
    }
  })

  const edges = plannedSteps.flatMap((step: any, index: number) => {
    const target = step.id || step.name || `step-${index}`
    return (step.depends_on || []).map((dep: string) => ({
      id: `${dep}->${target}`,
      source: dep,
      target,
      animated: ['running', 'process'].includes(String(stepStatus.get(target)?.status || '').toLowerCase()),
    }))
  })

  return { nodes, edges }
}

export const middlewareByID = (spec: DeployFlowSpec, id: DeployFlowMiddlewareNode['id']) =>
  spec.middleware.find((item) => item.id === id)

export const toolByID = (spec: DeployFlowSpec, id: DeployFlowToolNode['id']) =>
  spec.tools.find((item) => item.id === id)

export const enabledMiddleware = (spec: DeployFlowSpec, id: DeployFlowMiddlewareNode['id']) =>
  middlewareByID(spec, id)?.enabled === true

export const roleLabel = (role: string) => {
  const labels: Record<string, string> = {
    manager: 'Manager',
    master: 'Master',
    replica: 'Replica',
    primary: 'Primary',
    secondary: 'Secondary',
    bootstrap: 'Bootstrap',
  }
  return labels[role] || role
}

const architectureCustom = (spec: DeployFlowSpec) => {
  const custom = { ...(spec.custom || {}) }
  if (spec.arch === 'mgr' && !custom.group_name) custom.group_name = createMgrGroupName()
  if (spec.arch === 'pxc') {
    if (!custom.cluster_name) custom.cluster_name = spec.cluster_id
    if (!custom.sst_method) custom.sst_method = 'xtrabackup-v2'
    if (!custom.wsrep_port) custom.wsrep_port = 4567
  }
  return custom
}

const keepalivedPayload = (spec: DeployFlowSpec) => {
  const node = middlewareByID(spec, 'keepalived')
  return {
    enabled: node?.enabled === true,
    vip: node?.vip,
    vip_interface: node?.vip_interface || 'eth0',
  }
}

const proxysqlPayload = (spec: DeployFlowSpec) => {
  const node = middlewareByID(spec, 'proxysql')
  const agentPort = node?.agent_port || node?.proxy_agent_port || 9090
  return {
    enabled: node?.enabled === true,
    proxy_host_id: node?.proxy_host_id,
    proxy_host: node?.proxy_host,
    proxy_port: node?.proxy_port || 6033,
    agent_port: agentPort,
    proxy_agent_port: agentPort,
  }
}

const toolPayload = (spec: DeployFlowSpec, id: DeployFlowToolNode['id']) => {
  const node = toolByID(spec, id)
  return {
    enabled: node?.enabled === true,
    backup_type: node?.backup_type,
  }
}

const parseMySQLConfig = (text?: string): Record<string, string> => {
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

const statusStyle = (status?: string) => {
  const normalized = String(status || '').toLowerCase()
  if (['completed', 'success', 'succeeded', 'ok'].includes(normalized)) return { borderColor: '#52c41a', background: '#f6ffed' }
  if (['failed', 'error'].includes(normalized)) return { borderColor: '#ff4d4f', background: '#fff2f0' }
  if (['running', 'process'].includes(normalized)) return { borderColor: '#1677ff', background: '#e6f4ff' }
  if (['partial', 'partial_success'].includes(normalized)) return { borderColor: '#faad14', background: '#fffbe6' }
  return { borderColor: '#d9d9d9', background: '#fff' }
}
