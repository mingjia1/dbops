import { describe, expect, it } from 'vitest'
import {
  roleColor, statusColor, endpointOf, relationLabel, parseSlaveIds, inferEdgesFromInstances,
  primaryRoles, replicaRoles,
} from './topologyHelpers'
import type { Instance } from './api'

// ---- roleColor ----
describe('roleColor', () => {
  it('returns blue for primary roles', () => {
    expect(roleColor('master')).toBe('blue')
    expect(roleColor('primary')).toBe('blue')
    expect(roleColor('bootstrap')).toBe('blue')
  })
  it('returns green for replica roles', () => {
    expect(roleColor('slave')).toBe('green')
    expect(roleColor('replica')).toBe('green')
    expect(roleColor('secondary')).toBe('green')
  })
  it('returns purple for manager', () => {
    expect(roleColor('manager')).toBe('purple')
  })
  it('returns default for unknown', () => {
    expect(roleColor('arbiter')).toBe('default')
    expect(roleColor()).toBe('default')
  })
})

// ---- statusColor ----
describe('statusColor', () => {
  it('returns success for healthy statuses', () => {
    expect(statusColor('healthy')).toBe('success')
    expect(statusColor('running')).toBe('success')
    expect(statusColor('success')).toBe('success')
  })
  it('returns error for failed statuses', () => {
    expect(statusColor('failed')).toBe('error')
    expect(statusColor('stopped')).toBe('error')
    expect(statusColor('unhealthy')).toBe('error')
  })
  it('returns default for unknown', () => {
    expect(statusColor('pending')).toBe('default')
    expect(statusColor()).toBe('default')
  })
})

// ---- endpointOf ----
describe('endpointOf', () => {
  it('formats host:port from connection', () => {
    expect(endpointOf({ connection: { host: '10.0.0.1', port: 3306 } } as Instance)).toBe('10.0.0.1:3306')
  })
  it('falls back to host/port directly', () => {
    expect(endpointOf({ host: '10.0.0.1', port: 3306 } as Instance)).toBe('10.0.0.1:3306')
  })
  it('returns - for missing values', () => {
    expect(endpointOf({} as Instance)).toBe('-:-')
  })
  it('returns - for undefined', () => {
    expect(endpointOf()).toBe('-:-')
  })
})

// ---- relationLabel ----
describe('relationLabel', () => {
  it('maps pxc', () => { expect(relationLabel('pxc')).toBe('集群同步') })
  it('maps async', () => { expect(relationLabel('async')).toBe('异步复制') })
  it('maps semi-sync', () => { expect(relationLabel('semi-sync')).toBe('半同步') })
  it('maps mgr', () => { expect(relationLabel('mgr')).toBe('组复制') })
  it('maps group_replication', () => { expect(relationLabel('group_replication')).toBe('组复制') })
  it('maps replication', () => { expect(relationLabel('replication')).toBe('复制') })
  it('defaults to 复制', () => { expect(relationLabel()).toBe('复制') })
})

// ---- parseSlaveIds ----
describe('parseSlaveIds', () => {
  it('parses JSON array', () => {
    expect(parseSlaveIds('["id1","id2"]')).toEqual(['id1', 'id2'])
  })
  it('parses comma-separated', () => {
    expect(parseSlaveIds('id1, id2')).toEqual(['id1', 'id2'])
  })
  it('returns empty for undefined', () => {
    expect(parseSlaveIds()).toEqual([])
  })
  it('returns empty for empty string', () => {
    expect(parseSlaveIds('')).toEqual([])
  })
})

// ---- inferEdgesFromInstances ----
describe('inferEdgesFromInstances', () => {
  const base = (overrides: any = {}): Instance => ({
    id: 'i1', name: 'i1', cluster_id: 'c1',
    host: '10.0.0.1', port: 3306,
    ...overrides,
  } as Instance)

  it('returns empty for empty input', () => {
    expect(inferEdgesFromInstances([])).toEqual([])
  })

  it('infers edges from topology', () => {
    const master = base({ id: 'm1', topology: { master_id: '' } })
    const slave = base({ id: 's1', topology: { master_id: 'm1' } })
    const edges = inferEdgesFromInstances([master, slave])
    expect(edges.length).toBeGreaterThanOrEqual(1)
    expect(edges[0].source_id).toBe('m1')
    expect(edges[0].target_id).toBe('s1')
  })

  it('falls back to first instance as primary', () => {
    const a = base({ id: 'a', status: { role: 'unknown', health_status: 'healthy' }, topology: {} })
    const b = base({ id: 'b', status: { role: 'unknown', health_status: 'healthy' }, topology: {} })
    const edges = inferEdgesFromInstances([a, b])
    expect(edges.length).toBeGreaterThanOrEqual(1)
    expect(edges[0].source_id).toBe('a')
  })
})
