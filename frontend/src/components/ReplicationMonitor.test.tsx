import { render, screen } from '@testing-library/react'
import { beforeAll, describe, expect, it, vi } from 'vitest'
import ReplicationMonitor from './ReplicationMonitor'

describe('ReplicationMonitor', () => {
  beforeAll(() => {
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    })
  })

  it('shows HA/MHA replica fields as running', () => {
    render(
      <ReplicationMonitor
        status={{
          cluster_type: 'mha',
          Replica_IO_Running: 'Yes',
          Replica_SQL_Running: 'Yes',
          Seconds_Behind_Source: 0,
          Source_Host: '10.1.81.21',
          Source_Port: 3306,
        }}
      />
    )

    expect(screen.getAllByText('Running').length).toBeGreaterThanOrEqual(2)
  })

  it('does not treat a No string as running', () => {
    render(
      <ReplicationMonitor
        status={{
          cluster_type: 'ha',
          slave_io_running: 'No',
          slave_sql_running: 'Yes',
          seconds_behind_master: 0,
        }}
      />
    )

    expect(screen.getAllByText('Stopped').length).toBeGreaterThanOrEqual(1)
  })
})
