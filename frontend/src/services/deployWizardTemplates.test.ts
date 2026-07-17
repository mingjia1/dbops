import { describe, expect, it } from 'vitest'
import {
  WIZARD_TEMPLATES,
  buildHaWizardPayload,
  buildSingleInstanceCreate,
  buildWizardSummary,
  generateRootPassword,
  getWizardTemplate,
  makeWizardName,
} from './deployWizardTemplates'

describe('deployWizardTemplates', () => {
  it('has three beginner scenarios', () => {
    expect(WIZARD_TEMPLATES).toHaveLength(3)
    expect(getWizardTemplate('dev-single').mode).toBe('single')
    expect(getWizardTemplate('prod-ha').minHosts).toBe(2)
  })

  it('builds single instance create body with safe defaults', () => {
    const body = buildSingleInstanceCreate({
      hostId: 'h1',
      hostAddress: '10.0.0.1',
      password: 'Secret1!',
      name: 'wz_test',
    })
    expect(body.host_id).toBe('h1')
    expect(body.host).toBe('10.0.0.1')
    expect(body.port).toBe(3306)
    expect(body.version_id).toBe('mysql-8.0.36')
    expect(body.basedir).toContain('/opt/mysql')
    expect(body.datadir).toContain('/data/mysql')
    expect(body.password).toBe('Secret1!')
  })

  it('builds HA payload with master + replicas', () => {
    const payload = buildHaWizardPayload({
      masterHostId: 'm1',
      replicaHostIds: ['r1', 'r2'],
      password: 'Secret1!',
      clusterId: 'wz_ha_1',
    })
    expect(payload.cluster_type).toBe('ha')
    expect(payload.cluster_id).toBe('wz_ha_1')
    expect(payload.mysql?.password).toBe('Secret1!')
    const roles = (payload.nodes || []).map((n: any) => n.role)
    expect(roles).toContain('master')
    expect(roles.filter((r: string) => r === 'replica').length).toBe(2)
  })

  it('summary and helpers work', () => {
    expect(makeWizardName('dev-single', 1)).toContain('wz_')
    expect(generateRootPassword(12)).toHaveLength(12)
    const lines = buildWizardSummary({
      scenario: 'prod-single',
      hostLabels: ['10.0.0.2'],
      port: 3306,
      password: 'x',
    })
    expect(lines.some((l) => l.includes('生产单机'))).toBe(true)
  })
})
