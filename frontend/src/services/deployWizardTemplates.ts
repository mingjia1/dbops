/**
 * 小白部署向导：场景模板 + payload 构建（纯函数，可单测）。
 * 隐藏 GTID/MGR/路径等细节，只暴露业务场景。
 */

import { buildDeployPayload, type ArchType } from './deployHelpers'

export type WizardScenario = 'dev-single' | 'prod-single' | 'prod-ha'

export interface WizardTemplate {
  id: WizardScenario
  title: string
  desc: string
  /** 需要几台主机 */
  minHosts: number
  maxHosts: number
  /** 是否走集群 HA 部署（否则单实例 create+deploy） */
  mode: 'single' | 'ha'
  /** 面向用户的一句话说明 */
  summary: string
}

export const WIZARD_TEMPLATES: WizardTemplate[] = [
  {
    id: 'dev-single',
    title: '开发测试库',
    desc: '1 台机器，快速起一个 MySQL，适合开发联调',
    minHosts: 1,
    maxHosts: 1,
    mode: 'single',
    summary: '单机 MySQL 8.0 · 端口 3306 · 自动路径',
  },
  {
    id: 'prod-single',
    title: '生产单机',
    desc: '1 台机器，带生产默认参数思路，适合内部业务',
    minHosts: 1,
    maxHosts: 1,
    mode: 'single',
    summary: '单机 MySQL 8.0 · 生产路径 · 后续可加备份',
  },
  {
    id: 'prod-ha',
    title: '生产双机 HA',
    desc: '至少 2 台机器：主库 + 从库，尽量少停机',
    minHosts: 2,
    maxHosts: 8,
    mode: 'ha',
    summary: '主从复制 HA · MySQL 8.0 · 自动配复制',
  },
]

export interface WizardDefaults {
  mysqlVersion: string
  versionId: string
  port: number
  basedir: string
  datadir: string
  username: string
  osUser: string
}

/** 固定小白默认值；高级用户去「集群部署」改。 */
export const WIZARD_DEFAULTS: WizardDefaults = {
  mysqlVersion: '8.0.36',
  versionId: 'mysql-8.0.36',
  port: 3306,
  basedir: '/opt/mysql-8.0.36',
  datadir: '/data/mysql/3306',
  username: 'root',
  osUser: 'mysql',
}

export function getWizardTemplate(id: WizardScenario): WizardTemplate {
  const t = WIZARD_TEMPLATES.find((x) => x.id === id)
  if (!t) throw new Error(`unknown scenario: ${id}`)
  return t
}

/** 生成可读集群/实例名 */
export function makeWizardName(scenario: WizardScenario, stamp = Date.now()): string {
  const tag = scenario.replace(/-/g, '_')
  return `wz_${tag}_${stamp}`
}

/** 随机 root 密码（够用即可，非密码学库） */
export function generateRootPassword(len = 16): string {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$'
  let out = ''
  for (let i = 0; i < len; i++) {
    out += chars[Math.floor(Math.random() * chars.length)]
  }
  return out
}

export interface SingleDeployInput {
  hostId: string
  hostAddress: string
  password: string
  name?: string
  port?: number
}

/** 单机：instanceApi.create 入参 */
export function buildSingleInstanceCreate(input: SingleDeployInput) {
  const d = WIZARD_DEFAULTS
  const name = input.name || makeWizardName('dev-single')
  return {
    name,
    host: input.hostAddress,
    port: input.port || d.port,
    username: d.username,
    password: input.password,
    host_id: input.hostId,
    cluster_id: name,
    version_id: d.versionId,
    basedir: d.basedir,
    datadir: d.datadir,
    os_user: d.osUser,
  }
}

export interface HaDeployInput {
  masterHostId: string
  replicaHostIds: string[]
  password: string
  clusterId?: string
  port?: number
}

/** HA：复用 buildDeployPayload，架构固定 ha */
export function buildHaWizardPayload(input: HaDeployInput): Record<string, any> {
  const d = WIZARD_DEFAULTS
  const clusterId = input.clusterId || makeWizardName('prod-ha')
  const values = {
    cluster_id: clusterId,
    master_host_id: input.masterHostId,
    replica_host_ids: input.replicaHostIds,
    mysql_version: d.mysqlVersion,
    mysql_port: input.port || d.port,
    replica_port: input.port || d.port,
    master_data_dir: d.datadir,
    replica_data_dir: d.datadir,
    enable_proxysql: false,
    enable_health_check: true,
    enable_backup_tool: true,
  }
  const arch: ArchType = 'ha'
  return buildDeployPayload(arch, values, { username: d.username, password: input.password })
}

/** 向导摘要文案（确认页） */
export function buildWizardSummary(opts: {
  scenario: WizardScenario
  hostLabels: string[]
  port: number
  password: string
}): string[] {
  const t = getWizardTemplate(opts.scenario)
  return [
    `场景：${t.title}`,
    `说明：${t.summary}`,
    `主机：${opts.hostLabels.join('、') || '（未选）'}`,
    `端口：${opts.port}`,
    `账号：${WIZARD_DEFAULTS.username}`,
    `密码：${opts.password ? '（已设置）' : '（未设置）'}`,
    `版本：MySQL ${WIZARD_DEFAULTS.mysqlVersion}`,
  ]
}
