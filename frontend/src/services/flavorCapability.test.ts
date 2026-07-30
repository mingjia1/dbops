import { describe, it, expect } from 'vitest'
import {
  hasCapability,
  capabilityDisabledReason,
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
  'oceanbase',
  'gaussdb-mysql',
  'polardb-mysql',
  'tdsql-mysql',
]

const PG_COMPATIBLE_FLAVORS = ['kingbase', 'opengauss', 'highgo', 'gbase8a', 'shentong']
const DRIVERLESS_FLAVORS = ['dm', 'gbase8s']

describe('hasCapability', () => {
  it('grants every capability to MySQL-protocol engines', () => {
    for (const flavor of MYSQL_FLAVORS) {
      for (const capability of ALL_CAPABILITIES) {
        expect(hasCapability(flavor, capability)).toBe(true)
      }
    }
  })

  it('treats empty and unknown flavors as MySQL-compatible', () => {
    // Instances registered before flavor persistence carry an empty flavor and
    // must keep every action available.
    for (const flavor of [undefined, '', '   ', 'some-future-engine']) {
      for (const capability of ALL_CAPABILITIES) {
        expect(hasCapability(flavor, capability)).toBe(true)
      }
    }
  })

  it('refuses every MySQL-specific operation for PG-compatible engines but allows SQL health check', () => {
    for (const flavor of PG_COMPATIBLE_FLAVORS) {
      expect(hasCapability(flavor, 'health_sql')).toBe(true)
      for (const capability of ALL_CAPABILITIES.filter((c) => c !== 'health_sql')) {
        expect(hasCapability(flavor, capability)).toBe(false)
      }
    }
  })

  it('refuses every capability including SQL health check for driverless engines', () => {
    for (const flavor of DRIVERLESS_FLAVORS) {
      for (const capability of ALL_CAPABILITIES) {
        expect(hasCapability(flavor, capability)).toBe(false)
      }
    }
  })

  it('normalizes flavor case and surrounding whitespace', () => {
    expect(hasCapability('  KingBase  ', 'health_sql')).toBe(true)
    expect(hasCapability('  KingBase  ', 'failover')).toBe(false)
    expect(hasCapability('GBase8S', 'health_sql')).toBe(false)
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
    for (const flavor of [...PG_COMPATIBLE_FLAVORS, ...DRIVERLESS_FLAVORS]) {
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
    for (const flavor of [...PG_COMPATIBLE_FLAVORS, ...MYSQL_FLAVORS, '', undefined]) {
      expect(isDriverlessFlavor(flavor)).toBe(false)
    }
  })
})
