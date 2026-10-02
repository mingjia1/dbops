import { describe, expect, it } from 'vitest'
import {
  findOpenKeys,
  findSelectedKey,
  getDashboardMenuItems,
  type MenuItem,
} from './dashboardMenu'

const items = getDashboardMenuItems(true)

describe('getDashboardMenuItems', () => {
  it('exposes every route as a menu entry', () => {
    const keys: string[] = []
    const collect = (list: typeof items) => {
      for (const item of list) {
        if (!item || typeof item === 'number') continue
        keys.push(String(item.key))
        if ('children' in item && item.children) {
          collect(item.children as typeof items)
        }
      }
    }
    collect(items)

    // Every App.tsx page route that is meant to be reachable from the sidebar.
    const pageRoutes = [
      '/dashboard/monitor',
      '/dashboard/home',
      '/dashboard/hosts',
      '/dashboard/instances',
      '/dashboard/capability-matrix',
      '/dashboard/env-check',
      '/dashboard/backup',
      '/dashboard/cluster-deploy',
      '/dashboard/deploy-wizard',
      '/dashboard/ha',
      '/dashboard/role-switch',
      '/dashboard/migration',
      '/dashboard/migration-orchestrate',
      '/dashboard/upgrade',
      '/dashboard/arch-upgrade',
      '/dashboard/topology',
      '/dashboard/inspection',
      '/dashboard/ai-diagnosis',
      '/dashboard/fault-injection',
      '/dashboard/drills',
      '/dashboard/alert-rules',
      '/dashboard/alert-manage',
      '/dashboard/approvals',
      '/dashboard/audit-logs',
      '/dashboard/audit-verify',
      '/dashboard/masking',
      '/dashboard/key-rotation',
      '/dashboard/license',
      '/dashboard/security-settings',
      '/dashboard/data-storage',
      '/dashboard/agent-manage',
      '/dashboard/plugins',
      '/dashboard/users',
      '/dashboard/parameter-templates',
      '/dashboard/parameter-enhanced',
    ]
    for (const route of pageRoutes) {
      expect(keys, `missing menu entry for ${route}`).toContain(route)
    }
  })

  it('hides the user management entry when the flag is false', () => {
    const withUsers = getDashboardMenuItems(true)
    const withoutUsers = getDashboardMenuItems(false)
    const flat = (list: MenuItem[]) =>
      list.flatMap((item) => {
        if (!item || typeof item === 'number') return []
        if ('children' in item && item.children) {
          return (item.children as MenuItem[])
            .filter((child): boolean => !!child && typeof child !== 'number')
            .map((child) => String((child as unknown as Record<string, unknown>).key))
        }
        return [String((item as unknown as Record<string, unknown>).key)]
      })
    expect(flat(withUsers)).toContain('/dashboard/users')
    expect(flat(withoutUsers)).not.toContain('/dashboard/users')
  })
})

describe('findSelectedKey', () => {
  it('matches exact paths', () => {
    expect(findSelectedKey('/dashboard/home', items)).toBe('/dashboard/home')
    expect(findSelectedKey('/dashboard/topology', items)).toBe('/dashboard/topology')
  })

  it('does not let /dashboard/migration swallow /dashboard/migration-orchestrate', () => {
    expect(findSelectedKey('/dashboard/migration-orchestrate', items)).toBe(
      '/dashboard/migration-orchestrate',
    )
    expect(findSelectedKey('/dashboard/migration', items)).toBe('/dashboard/migration')
  })

  it('does not let /dashboard/deploy swallow /dashboard/deploy-wizard', () => {
    expect(findSelectedKey('/dashboard/deploy-wizard', items)).toBe('/dashboard/deploy-wizard')
    expect(findSelectedKey('/dashboard/cluster-deploy', items)).toBe('/dashboard/cluster-deploy')
  })

  it('does not let /dashboard/security swallow /dashboard/security-settings', () => {
    expect(findSelectedKey('/dashboard/security-settings', items)).toBe(
      '/dashboard/security-settings',
    )
  })

  it('falls back to the longest prefix for nested detail routes', () => {
    expect(findSelectedKey('/dashboard/hosts/abc-123', items)).toBe('/dashboard/hosts')
    expect(findSelectedKey('/dashboard/instances/xyz/edit', items)).toBe('/dashboard/instances')
  })

  it('defaults to home for unknown paths', () => {
    expect(findSelectedKey('/dashboard/does-not-exist', items)).toBe('/dashboard/home')
  })
})

describe('findOpenKeys', () => {
  it('opens the parent group of the selected entry', () => {
    expect(findOpenKeys('/dashboard/migration-orchestrate', items)).toEqual([
      '/dashboard/migration-upgrade',
    ])
    expect(findOpenKeys('/dashboard/alert-manage', items)).toEqual(['/dashboard/alert-center'])
    expect(findOpenKeys('/dashboard/license', items)).toEqual(['/dashboard/security'])
    expect(findOpenKeys('/dashboard/hosts', items)).toEqual(['/dashboard/resources'])
  })
})
