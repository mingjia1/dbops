import { render, screen, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProtectedRoute from './ProtectedRoute'
import { authApi } from '../services/api'

vi.mock('../services/api', () => ({
  authApi: {
    me: vi.fn(),
  },
}))

describe('ProtectedRoute', () => {
  const renderRoute = (element: ReactNode) => render(
    <MemoryRouter
      initialEntries={['/dashboard/home']}
      future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
    >
      <Routes>
        <Route path="/dashboard/home" element={element} />
        <Route path="/login" element={<div>login page</div>} />
      </Routes>
    </MemoryRouter>
  )

  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('allows cookie-backed sessions without a local token', async () => {
    vi.mocked(authApi.me).mockResolvedValue({ data: { username: 'tester' } } as any)

    renderRoute(
      <ProtectedRoute>
        <div>protected content</div>
      </ProtectedRoute>
    )

    expect(await screen.findByText('protected content')).toBeInTheDocument()
    expect(localStorage.getItem('user')).toContain('tester')
  })

  it('rejects invalid local tokens before trusting stale client state', async () => {
    localStorage.setItem('token', 'short-token')

    renderRoute(
      <ProtectedRoute>
        <div>protected content</div>
      </ProtectedRoute>
    )

    await waitFor(() => expect(screen.getByText('login page')).toBeInTheDocument())
    expect(authApi.me).not.toHaveBeenCalled()
    expect(localStorage.getItem('token')).toBeNull()
  })

  it('redirects to login when the session check fails', async () => {
    vi.mocked(authApi.me).mockRejectedValue(new Error('unauthorized'))

    renderRoute(
      <ProtectedRoute>
        <div>protected content</div>
      </ProtectedRoute>
    )

    await waitFor(() => expect(screen.getByText('login page')).toBeInTheDocument())
  })
})
