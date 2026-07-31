/**
 * Frontend mirror of the backend engine capability table.
 *
 * Source of truth: backend/internal/services/flavor_capability.go
 * The backend gate is authoritative and rejects unsupported operations
 * regardless of what the UI shows. This table exists only so the UI can
 * disable actions that would certainly be refused, instead of letting users
 * click through to an error.
 *
 * When adding a flavor or capability, update the Go table first.
 */

export type Capability =
  | 'replication'
  | 'failover'
  | 'cluster_deploy'
  | 'backup_physical'
  | 'upgrade_inplace'
  | 'health_sql'
  | 'scale'
  | 'node_rebuild'
  | 'instance_deploy'
  | 'upgrade_logical'
  | 'parameter_template'
  | 'instance_admin'

export interface FlavorAwareInstance {
  version?: { flavor?: string }
}

export const CAPABILITY_LABELS: Record<Capability, string> = {
  replication: '主从复制搭建',
  failover: '故障切换',
  cluster_deploy: '集群架构部署',
  backup_physical: '物理备份/恢复 (xtrabackup)',
  upgrade_inplace: '原地版本升级',
  health_sql: 'SQL 连接健康检测',
  scale: '集群节点扩缩容',
  node_rebuild: '节点重建',
  instance_deploy: '单实例部署',
  upgrade_logical: '逻辑迁移升级',
  parameter_template: '参数模板下发',
  instance_admin: '实例管理操作 (账号/参数/配置/服务控制)',
}

/** Engines speaking the MySQL wire protocol and supporting MySQL admin SQL get every capability. */
const MYSQL_PROTOCOL_FLAVORS = [
  'mysql',
  'mariadb',
  'percona',
  'oceanbase',
  'gaussdb-mysql',
  'polardb-mysql',
  'tdsql-mysql',
]

/**
 * Engines onboarded for inventory, health and topology only. The boolean is
 * whether a SQL health check is possible: engines with no usable Go driver can
 * only be probed over TCP.
 */
const TIERED_ONBOARDING_FLAVORS: Record<string, { healthSql: boolean }> = {
  // TiDB speaks the MySQL protocol but has distinct replication, backup and
  // lifecycle workflows, so the MySQL connector is limited to health checks.
  tidb: { healthSql: true },
  kingbase: { healthSql: true },
  opengauss: { healthSql: true },
  highgo: { healthSql: true },
  gbase8a: { healthSql: true },
  shentong: { healthSql: true },
  // No pure-Go driver: proprietary (dm) and Informix (gbase8s) protocols.
  dm: { healthSql: false },
  gbase8s: { healthSql: false },
}

const normalizeFlavor = (flavor?: string): string => {
  const normalized = (flavor || '').trim().toLowerCase()
  return normalized === '' ? 'mysql' : normalized
}

/**
 * Reports whether an engine supports an operation.
 *
 * Unknown and empty flavors are treated as MySQL-compatible, matching
 * HasCapability in the Go table. Instances registered before flavor
 * persistence carry an empty flavor and must keep every action available.
 */
export const hasCapability = (flavor: string | undefined, capability: Capability): boolean => {
  const normalized = normalizeFlavor(flavor)
  if (MYSQL_PROTOCOL_FLAVORS.includes(normalized)) return true

  const tiered = TIERED_ONBOARDING_FLAVORS[normalized]
  if (!tiered) return true // unknown flavor: treat as MySQL-compatible

  return capability === 'health_sql' ? tiered.healthSql : false
}

/** Tests an instance's persisted engine flavor against an operation. */
export const instanceHasCapability = (instance: FlavorAwareInstance, capability: Capability): boolean =>
  hasCapability(instance.version?.flavor, capability)

/** Returns a tooltip explaining why an action is unavailable, or '' when it is. */
export const capabilityDisabledReason = (
  flavor: string | undefined,
  capability: Capability,
): string => {
  if (hasCapability(flavor, capability)) return ''
  return `数据库类型 ${normalizeFlavor(flavor)} 不支持${CAPABILITY_LABELS[capability]}，该类型仅支持纳管、健康检测与拓扑展示`
}

/** True when the engine is onboarded without MySQL-specific operations. */
export const isTieredOnboardingFlavor = (flavor?: string): boolean =>
  normalizeFlavor(flavor) in TIERED_ONBOARDING_FLAVORS

/** True when the engine has no Go driver and can only be probed over TCP. */
export const isDriverlessFlavor = (flavor?: string): boolean => {
  const tiered = TIERED_ONBOARDING_FLAVORS[normalizeFlavor(flavor)]
  return tiered !== undefined && !tiered.healthSql
}
