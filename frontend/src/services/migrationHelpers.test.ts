import { describe, expect, it } from 'vitest'
import {
  MIGRATION_SUBSTEPS, activeMigrationStatuses,
  isActiveMigrationStatus, isFailedMigrationStatus, isCompletedMigrationStatus,
  migrationProgressStatus, migrationStatusColor,
  formatBytes, buildProgressDetails, buildCreatePayload, taskFromResult, getCurrentSubstages,
  type MigrationProgressResponse,
} from './migrationHelpers'

// ---- MIGRATION_SUBSTEPS ----

describe('MIGRATION_SUBSTEPS', () => {
  it('has entries for all expected stages', () => {
    expect(MIGRATION_SUBSTEPS['数据导出']).toBeDefined()
    expect(MIGRATION_SUBSTEPS['数据传输']).toBeDefined()
    expect(MIGRATION_SUBSTEPS['数据导入']).toBeDefined()
    expect(MIGRATION_SUBSTEPS['一致性校验']).toBeDefined()
    expect(MIGRATION_SUBSTEPS['切换']).toBeDefined()
  })

  it('each stage has at least 2 substeps', () => {
    for (const [, substeps] of Object.entries(MIGRATION_SUBSTEPS)) {
      expect(substeps.length).toBeGreaterThanOrEqual(2)
    }
  })
})

describe('activeMigrationStatuses', () => {
  it('contains expected active statuses', () => {
    expect(activeMigrationStatuses.has('pending')).toBe(true)
    expect(activeMigrationStatuses.has('preparing')).toBe(true)
    expect(activeMigrationStatuses.has('migrating')).toBe(true)
    expect(activeMigrationStatuses.has('running')).toBe(true)
    expect(activeMigrationStatuses.has('verifying')).toBe(true)
    expect(activeMigrationStatuses.has('switching')).toBe(true)
  })

  it('does not contain terminal statuses', () => {
    expect(activeMigrationStatuses.has('completed')).toBe(false)
    expect(activeMigrationStatuses.has('failed')).toBe(false)
    expect(activeMigrationStatuses.has('cancelled')).toBe(false)
  })
})

// ---- isActiveMigrationStatus ----

describe('isActiveMigrationStatus', () => {
  it('returns true for active statuses', () => {
    expect(isActiveMigrationStatus('running')).toBe(true)
    expect(isActiveMigrationStatus('migrating')).toBe(true)
    expect(isActiveMigrationStatus('pending')).toBe(true)
  })

  it('returns false for terminal statuses', () => {
    expect(isActiveMigrationStatus('completed')).toBe(false)
    expect(isActiveMigrationStatus('failed')).toBe(false)
  })

  it('returns false for undefined', () => {
    expect(isActiveMigrationStatus()).toBe(false)
  })

  it('is case-insensitive', () => {
    expect(isActiveMigrationStatus('RUNNING')).toBe(true)
  })
})

// ---- isFailedMigrationStatus ----

describe('isFailedMigrationStatus', () => {
  it('returns true for failed variants', () => {
    expect(isFailedMigrationStatus('failed')).toBe(true)
    expect(isFailedMigrationStatus('error')).toBe(true)
    expect(isFailedMigrationStatus('timeout')).toBe(true)
    expect(isFailedMigrationStatus('cancelled')).toBe(true)
    expect(isFailedMigrationStatus('canceled')).toBe(true)
  })

  it('returns false for success', () => {
    expect(isFailedMigrationStatus('completed')).toBe(false)
  })

  it('returns false for undefined', () => {
    expect(isFailedMigrationStatus()).toBe(false)
  })
})

// ---- isCompletedMigrationStatus ----

describe('isCompletedMigrationStatus', () => {
  it('returns true for completed variants', () => {
    expect(isCompletedMigrationStatus('completed')).toBe(true)
    expect(isCompletedMigrationStatus('success')).toBe(true)
    expect(isCompletedMigrationStatus('succeeded')).toBe(true)
    expect(isCompletedMigrationStatus('ok')).toBe(true)
  })

  it('returns false for failed', () => {
    expect(isCompletedMigrationStatus('failed')).toBe(false)
  })

  it('returns false for undefined', () => {
    expect(isCompletedMigrationStatus()).toBe(false)
  })
})

// ---- migrationProgressStatus ----

describe('migrationProgressStatus', () => {
  it('returns success for completed', () => {
    expect(migrationProgressStatus('completed')).toBe('success')
  })

  it('returns exception for failed', () => {
    expect(migrationProgressStatus('failed')).toBe('exception')
  })

  it('returns active for running', () => {
    expect(migrationProgressStatus('running')).toBe('active')
  })

  it('returns normal for unknown', () => {
    expect(migrationProgressStatus('unknown')).toBe('normal')
  })

  it('returns normal for undefined', () => {
    expect(migrationProgressStatus()).toBe('normal')
  })
})

// ---- migrationStatusColor ----

describe('migrationStatusColor', () => {
  it('returns success for completed', () => {
    expect(migrationStatusColor('completed')).toBe('success')
    expect(migrationStatusColor('success')).toBe('success')
  })

  it('returns error for failed', () => {
    expect(migrationStatusColor('failed')).toBe('error')
    expect(migrationStatusColor('error')).toBe('error')
  })

  it('returns warning for verifying/switching', () => {
    expect(migrationStatusColor('verifying')).toBe('warning')
    expect(migrationStatusColor('switching')).toBe('warning')
  })

  it('returns processing for active', () => {
    expect(migrationStatusColor('running')).toBe('processing')
    expect(migrationStatusColor('migrating')).toBe('processing')
  })

  it('returns default for unknown', () => {
    expect(migrationStatusColor('unknown')).toBe('default')
  })
})

// ---- formatBytes ----

describe('formatBytes', () => {
  it('returns 0 B for zero', () => { expect(formatBytes(0)).toBe('0 B') })
  it('returns 0 B for undefined', () => { expect(formatBytes()).toBe('0 B') })
  it('returns 0 B for negative', () => { expect(formatBytes(-1)).toBe('0 B') })
  it('formats bytes', () => { expect(formatBytes(500)).toBe('500 B') })
  it('formats KB', () => { expect(formatBytes(2048)).toBe('2.0 KB') })
  it('formats MB', () => { expect(formatBytes(5 * 1024 * 1024)).toBe('5.0 MB') })
  it('formats GB', () => { expect(formatBytes(3 * 1024 * 1024 * 1024)).toBe('3.0 GB') })
  it('formats TB', () => {
    const tb = 2 * 1024 * 1024 * 1024 * 1024
    expect(formatBytes(tb)).toBe('2.0 TB')
  })
})

// ---- buildCreatePayload ----

describe('buildCreatePayload', () => {
  it('builds payload with correct fields', () => {
    const values = { source_instance: 'i1', target_instance: 'i2', extra: 'x' }
    const result = buildCreatePayload(values, 'physical')
    expect(result.source_instance_id).toBe('i1')
    expect(result.target_instance_id).toBe('i2')
    expect(result.strategy).toBe('physical')
    expect(result.name).toMatch(/^physical-\d+$/)
    expect(JSON.parse(result.config)).toEqual(values)
  })
})

// ---- taskFromResult ----

describe('taskFromResult', () => {
  it('creates MigrationTask from result', () => {
    const values = { source_instance: 'i1', target_instance: 'i2' }
    const res = { data: { task_id: 'task-1', status: 'running', progress: 50 } }
    const task = taskFromResult(values, 'physical', res)
    expect(task).not.toBeNull()
    expect(task!.id).toBe('task-1')
    expect(task!.strategy).toBe('physical')
    expect(task!.source_instance_id).toBe('i1')
    expect(task!.progress).toBe(50)
  })

  it('returns null for missing task_id', () => {
    expect(taskFromResult({}, 'physical', { data: {} })).toBeNull()
  })

  it('returns null for empty response', () => {
    expect(taskFromResult({}, 'physical', {})).toBeNull()
  })
})

// ---- getCurrentSubstages ----

describe('getCurrentSubstages', () => {
  it('returns verifying substeps for verifying', () => {
    expect(getCurrentSubstages('verifying')).toEqual(MIGRATION_SUBSTEPS['一致性校验'])
  })

  it('returns switch substeps for switching', () => {
    expect(getCurrentSubstages('switching')).toEqual(MIGRATION_SUBSTEPS['切换'])
  })

  it('returns default substeps for other statuses', () => {
    expect(getCurrentSubstages('migrating')).toEqual(MIGRATION_SUBSTEPS['数据导出'])
    expect(getCurrentSubstages('running')).toEqual(MIGRATION_SUBSTEPS['数据导出'])
  })
})

// ---- buildProgressDetails ----

describe('buildProgressDetails', () => {
  const defaultProgress: MigrationProgressResponse = {
    task_id: 't1', status: 'running', progress: 45,
    current_step: '数据导入', total_steps: 5, completed_steps: 2,
    data_transferred: 1024 * 1024 * 100,
    logs: [],
  }

  it('builds 3 progress steps', () => {
    const details = buildProgressDetails(defaultProgress)
    expect(details).toHaveLength(3)
  })

  it('includes current stage info', () => {
    const details = buildProgressDetails(defaultProgress)
    expect(details[0].stage).toBe('数据导入')
    expect(details[0].progress).toBe(45)
  })

  it('calculates step completion percentage', () => {
    const details = buildProgressDetails(defaultProgress)
    expect(details[1].progress).toBe(40) // 2/5 * 100
    expect(details[1].details).toContain('2/5')
  })

  it('formats data transferred', () => {
    const details = buildProgressDetails(defaultProgress)
    expect(details[2].details).toContain('100.0 MB')
  })

  it('handles zero total_steps gracefully', () => {
    const details = buildProgressDetails({ ...defaultProgress, total_steps: 0, completed_steps: 0 })
    expect(details[1].progress).toBe(45)
  })
})
