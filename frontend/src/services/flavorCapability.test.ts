import { describe, it, expect } from 'vitest'
import {
  hasCapability,
  capabilityDisabledReason,
  instanceHasCapability,
  isTieredOnboardingFlavor,
  isDriverlessFlavor,
  CAPABILITY_LABELS,
  type Capability,
} from './flavorCapability'

const ALL_CAPABILITIES = Object.keys(CAPABILITY_LABELS) as Capability[]

const MYSQL_FLAVORS = [
  'mysql',
  'mariadb',
  'percona',
]

const PG_COMPATIBLE_FLAVORS = ['kingbase', 'opengauss', 'highgo', 'gbase8a', 'shentong']
const MYSQL_PROTOCOL_TIERED_FLAVORS = ['gaussdb-mysql', 'polardb-mysql', 'tdsql-mysql']
const DRIVERLESS_FLAVORS = ['dm', 'gbase8s']
const COMPLETED_SINGLE_NODE_FLAVORS = ['oceanbase', 'tidb']

describe('hasCapability', () => {
  it('grants every capability to MySQL-protocol engines', () => {
    for (const flavor of MYSQL_FLAVORS) {
      for (const capability of ALL_CAPABILITIES) {
        expect(hasCapability(flavor, capability)).toBe(true)
      }
    }
  })

  it('preserves legacy empty flavors and restricts explicit unknown flavors', () => {
    // Instances registered before flavor persistence carry an empty flavor and
    // must keep every action available.
    for (const flavor of [undefined, '', '   ']) {
      for (const capability of ALL_CAPABILITIES) {
        expect(hasCapability(flavor, capability)).toBe(true)
      }
    }
    for (const flavor of ['unknown', 'non-mysql', 'tcp-only', 'some-future-engine']) {
      for (const capability of ALL_CAPABILITIES) {
        expect(hasCapability(flavor, capability)).toBe(false)
      }
    }
  })

  it('keeps dedicated flavor Agent operations out of generic console actions', () => {
    for (const flavor of [...PG_COMPATIBLE_FLAVORS, ...MYSQL_PROTOCOL_TIERED_FLAVORS]) {
      expect(hasCapability(flavor, 'health_sql')).toBe(true)
      for (const capability of ALL_CAPABILITIES.filter((c) => c !== 'health_sql')) {
        expect(hasCapability(flavor, capability)).toBe(false)
      }
    }
  })

  it('refuses instance admin operations for every tiered onboarding engine', () => {
    // The agent instance-admin family runs MySQL DDL/DCL and manages my.cnf.
    for (const flavor of [...PG_COMPATIBLE_FLAVORS, ...MYSQL_PROTOCOL_TIERED_FLAVORS, ...DRIVERLESS_FLAVORS]) {
      expect(hasCapability(flavor, 'instance_admin')).toBe(false)
    }
    for (const flavor of MYSQL_FLAVORS) {
      expect(hasCapability(flavor, 'instance_admin')).toBe(true)
    }
  })

  it('mirrors completed single-node executor capabilities', () => {
    const completed: Record<string, Capability[]> = {
      oceanbase: ['instance_deploy', 'parameter_template', 'backup_physical', 'upgrade_inplace'],
      tidb: ['instance_deploy', 'parameter_template', 'backup_physical', 'upgrade_inplace', 'instance_admin'],
    }

    for (const flavor of COMPLETED_SINGLE_NODE_FLAVORS) {
      for (const capability of completed[flavor]) {
        expect(hasCapability(flavor, capability)).toBe(true)
      }
      for (const capability of ['replication', 'failover', 'scale', 'node_rebuild', 'cluster_deploy'] as Capability[]) {
        expect(hasCapability(flavor, capability)).toBe(false)
      }
    }
  })

  it('refuses every capability including SQL health check for driverless engines without completed executors', () => {
    for (const flavor of DRIVERLESS_FLAVORS.filter((flavor) => flavor !== 'gbase8s')) {
      for (const capability of ALL_CAPABILITIES) {
        expect(hasCapability(flavor, capability)).toBe(false)
      }
    }
  })

  it('normalizes flavor case and surrounding whitespace', () => {
    expect(hasCapability('  KingBase  ', 'health_sql')).toBe(true)
    expect(hasCapability('  KingBase  ', 'failover')).toBe(false)
    expect(hasCapability('GBase8S', 'backup_physical')).toBe(false)

    expect(hasCapability(' TiDB ', 'health_sql')).toBe(true)
    expect(hasCapability(' TiDB ', 'backup_physical')).toBe(true)
    expect(hasCapability('DM', 'scale')).toBe(false)
  })
})

describe('capabilityDisabledReason', () => {
  it('returns an empty string when the capability is supported', () => {
    expect(capabilityDisabledReason('mysql', 'failover')).toBe('')
    expect(capabilityDisabledReason('', 'cluster_deploy')).toBe('')
    expect(capabilityDisabledReason('kingbase', 'health_sql')).toBe('')
  })

  it('names both the flavor and the operation when unsupported', () => {
    const reason = capabilityDisabledReason('gbase8s', 'failover')
    expect(reason).toContain('gbase8s')
    expect(reason).toContain(CAPABILITY_LABELS.failover)
  })
})

describe('flavor classification', () => {
  it('identifies tiered onboarding flavors', () => {
    for (const flavor of [...PG_COMPATIBLE_FLAVORS, ...MYSQL_PROTOCOL_TIERED_FLAVORS, ...COMPLETED_SINGLE_NODE_FLAVORS, ...DRIVERLESS_FLAVORS]) {
      expect(isTieredOnboardingFlavor(flavor)).toBe(true)
    }
    for (const flavor of [...MYSQL_FLAVORS, '', undefined]) {
      expect(isTieredOnboardingFlavor(flavor)).toBe(false)
    }
  })

  it('identifies driverless flavors', () => {
    for (const flavor of DRIVERLESS_FLAVORS) {
      expect(isDriverlessFlavor(flavor)).toBe(true)
    }
    for (const flavor of [...PG_COMPATIBLE_FLAVORS, ...MYSQL_PROTOCOL_TIERED_FLAVORS, ...MYSQL_FLAVORS, '', undefined]) {
      expect(isDriverlessFlavor(flavor)).toBe(false)
    }
  })
})

describe('instanceHasCapability', () => {
  it('uses the persisted instance flavor for UI operation gates', () => {
    expect(instanceHasCapability({ version: { flavor: 'tidb' } }, 'health_sql')).toBe(true)
    expect(instanceHasCapability({ version: { flavor: 'tidb' } }, 'backup_physical')).toBe(true)
    expect(instanceHasCapability({ version: { flavor: 'kingbase' } }, 'parameter_template')).toBe(false)
  })
})
