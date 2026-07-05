import React from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ClusterActions } from './ClusterActions'
import { clusterDeployApi, hostApi } from '../services/api'

vi.mock('../services/api', () => ({
  clusterDeployApi: {
    destroy: vi.fn(),
    rebuildNode: vi.fn(),
    scaleIn: vi.fn(),
    scaleOut: vi.fn(),
  },
  hostApi: {
    list: vi.fn(),
  },
}))

vi.mock('antd', async () => {
  const actual = await vi.importActual<typeof import('antd')>('antd')
  return {
    ...actual,
    message: {
      error: vi.fn(),
      success: vi.fn(),
      warning: vi.fn(),
    },
    Popconfirm: ({ children, onConfirm }: any) => React.cloneElement(children, { onClick: onConfirm }),
  }
})

describe('ClusterActions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(hostApi.list).mockResolvedValue({ data: [] } as any)
  })

  it('uses the deployment destroy API for cluster deletion', async () => {
    const onActionComplete = vi.fn()
    vi.mocked(clusterDeployApi.destroy).mockResolvedValue({ data: { status: 'destroyed' } } as any)

    render(<ClusterActions clusterID="cluster-1" onActionComplete={onActionComplete} />)

    fireEvent.click(screen.getByText('销毁'))

    await waitFor(() => {
      expect(clusterDeployApi.destroy).toHaveBeenCalledWith('cluster-1')
    })
    expect(clusterDeployApi.scaleOut).not.toHaveBeenCalled()
    expect(onActionComplete).toHaveBeenCalledWith('destroy', { status: 'destroyed' })
  })
})
