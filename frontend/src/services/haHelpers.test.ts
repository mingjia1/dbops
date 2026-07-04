import { describe, expect, it } from 'vitest'
import {
  haNodeID, haNodeEndpoint, instanceEndpoint,
  normalizeRole, isPrimaryRole, isReplicaRole,
  isFailedHAStatus, isCompletedHAStatus, isSkippedHAStatus, isPartialHAStatus,
  isMGRInstance, isPXCInstance, detectClusterArch,
} from './haHelpers'
import type { Instance } from './api'

// ---- haNodeID ----
describe('haNodeID', () => {
  it('uses instance_id first', () => { expect(haNodeID({ instance_id: 'i1' })).toBe('i1') })
  it('falls back to id', () => { expect(haNodeID({ id: 'i1' })).toBe('i1') })
  it('returns - for empty', () => { expect(haNodeID(null)).toBe('-') })
})

// ---- haNodeEndpoint ----
describe('haNodeEndpoint', () => {
  it('formats host:port', () => { expect(haNodeEndpoint({ host: '10.0.0.1', port: 3306 })).toBe('10.0.0.1:3306') })
  it('returns host only', () => { expect(haNodeEndpoint({ host: '10.0.0.1' })).toBe('10.0.0.1') })
  it('returns - for empty', () => { expect(haNodeEndpoint(null)).toBe('-') })
})

// ---- instanceEndpoint ----
describe('instanceEndpoint', () => {
  it('formats from Instance', () => {
    expect(instanceEndpoint({ host: '10.0.0.1', port: 3306 } as Instance)).toBe('10.0.0.1:3306')
  })
  it('returns - for undefined', () => { expect(instanceEndpoint()).toBe('-') })
})

// ---- normalizeRole ----
describe('normalizeRole', () => {
  it('lowercases', () => { expect(normalizeRole('MASTER')).toBe('master') })
  it('returns empty for undefined', () => { expect(normalizeRole()).toBe('') })
})

// ---- isPrimaryRole ----
describe('isPrimaryRole', () => {
  it('matches primary variants', () => {
    expect(isPrimaryRole('master')).toBe(true)
    expect(isPrimaryRole('primary')).toBe(true)
    expect(isPrimaryRole('replica')).toBe(false)
  })
})

// ---- isReplicaRole ----
describe('isReplicaRole', () => {
  it('matches replica variants', () => {
    expect(isReplicaRole('slave')).toBe(true)
    expect(isReplicaRole('secondary')).toBe(true)
    expect(isReplicaRole('master')).toBe(false)
  })
})

// ---- Status helpers ----
describe('isFailedHAStatus', () => {
  it('returns true for failed/error/timeout', () => {
    expect(isFailedHAStatus('failed')).toBe(true)
    expect(isFailedHAStatus('error')).toBe(true)
    expect(isFailedHAStatus('timeout')).toBe(true)
  })
  it('returns false for success', () => { expect(isFailedHAStatus('success')).toBe(false) })
})

describe('isCompletedHAStatus', () => {
  it('returns true for success/completed', () => {
    expect(isCompletedHAStatus('success')).toBe(true)
    expect(isCompletedHAStatus('completed')).toBe(true)
  })
  it('returns false for failed', () => { expect(isCompletedHAStatus('failed')).toBe(false) })
})

describe('isSkippedHAStatus', () => {
  it('returns true for skipped', () => { expect(isSkippedHAStatus('skipped')).toBe(true) })
  it('returns false otherwise', () => { expect(isSkippedHAStatus('pending')).toBe(false) })
})

describe('isPartialHAStatus', () => {
  it('returns true for partial_success', () => { expect(isPartialHAStatus('partial_success')).toBe(true) })
  it('returns false otherwise', () => { expect(isPartialHAStatus('partial')).toBe(false) })
})

// ---- Architecture detection ----
describe('isMGRInstance', () => {
  const mgr = (mode: string) => ({ topology: { replication_mode: mode }, status: { replication_status: '' } } as Instance)
  it('detects mgr mode', () => { expect(isMGRInstance(mgr('mgr'))).toBe(true) })
  it('detects group_replication', () => { expect(isMGRInstance(mgr('group_replication'))).toBe(true) })
  it('rejects async', () => { expect(isMGRInstance(mgr('async'))).toBe(false) })
})

describe('isPXCInstance', () => {
  const pxc = (mode: string) => ({ topology: { replication_mode: mode }, status: { replication_status: '' } } as Instance)
  it('detects pxc mode', () => { expect(isPXCInstance(pxc('pxc'))).toBe(true) })
  it('detects galera', () => { expect(isPXCInstance(pxc('galera'))).toBe(true) })
  it('rejects async', () => { expect(isPXCInstance(pxc('async'))).toBe(false) })
})

describe('detectClusterArch', () => {
  const inst = (mode: string) => ({ topology: { replication_mode: mode } } as Instance)

  it('detects mgr', () => { expect(detectClusterArch([inst('mgr')])).toBe('mgr') })
  it('detects pxc', () => { expect(detectClusterArch([inst('pxc')])).toBe('pxc') })
  it('detects mha', () => { expect(detectClusterArch([inst('mha')])).toBe('mha') })
  it('detects ha', () => { expect(detectClusterArch([inst('async')])).toBe('ha') })
  it('returns empty for empty array', () => { expect(detectClusterArch([])).toBe('') })
})
