import React from 'react'
import {
  AlertOutlined, ApartmentOutlined, AuditOutlined, BarChartOutlined,
  CloudOutlined, ClusterOutlined, DashboardOutlined, DatabaseOutlined, DesktopOutlined,
  FileTextOutlined, HddOutlined, HeartOutlined,
  AppstoreOutlined, RetweetOutlined, RocketOutlined, SettingOutlined, SwapOutlined, PartitionOutlined,
  SafetyOutlined, UserOutlined, ExperimentOutlined, ThunderboltOutlined, RobotOutlined, KeyOutlined,
  FileSearchOutlined, LockOutlined, CopyrightOutlined,
} from '@ant-design/icons'
import type { MenuProps } from 'antd'

export type MenuItem = Required<MenuProps>['items'][number]

/**
 * Sidebar menu covering every route registered in App.tsx.
 *
 * Keys MUST stay in sync with the `path` props in App.tsx. `findSelectedKey`
 * matches exact first, then falls back to `pathname.startsWith`. The fallback
 * means a key must not be a prefix of a sibling key that is meant to be
 * selected separately (nested routes like `hosts/:id/edit` are intentionally
 * excluded — they are reached from list pages, not the sidebar).
 */
export const getDashboardMenuItems = (hasUserManage: boolean): MenuItem[] => [
  { key: '/dashboard/monitor', icon: <BarChartOutlined />, label: '监控仪表盘' },
  { key: '/dashboard/platform-health', icon: <HeartOutlined />, label: '平台健康' },
  { key: '/dashboard/home', icon: <DashboardOutlined />, label: '总览' },
  { key: '/dashboard/first-deploy', icon: <RocketOutlined />, label: '快速部署' },
  {
    key: '/dashboard/resources',
    icon: <DesktopOutlined />,
    label: '主机与实例',
    children: [
      { key: '/dashboard/hosts', icon: <DesktopOutlined />, label: '主机管理' },
      { key: '/dashboard/instances', icon: <DatabaseOutlined />, label: '实例管理' },
      { key: '/dashboard/capability-matrix', icon: <DatabaseOutlined />, label: '引擎能力矩阵' },
    ],
  },
  { key: '/dashboard/env-check', icon: <SettingOutlined />, label: '环境检查' },
  { key: '/dashboard/backup', icon: <CloudOutlined />, label: '备份管理' },
  {
    key: '/dashboard/deploy',
    icon: <RocketOutlined />,
    label: '集群部署',
    children: [
      { key: '/dashboard/cluster-deploy', icon: <ClusterOutlined />, label: '集群部署 - 流程编排' },
      { key: '/dashboard/deploy-wizard', icon: <RocketOutlined />, label: '集群部署 - 表单' },
    ],
  },
  { key: '/dashboard/ha', icon: <HeartOutlined />, label: '高可用管理' },
  { key: '/dashboard/role-switch', icon: <RetweetOutlined />, label: '角色切换' },
  {
    key: '/dashboard/migration-upgrade',
    icon: <SwapOutlined />,
    label: '迁移与升级',
    children: [
      { key: '/dashboard/migration', icon: <PartitionOutlined />, label: '数据迁移' },
      { key: '/dashboard/migration-orchestrate', icon: <PartitionOutlined />, label: '迁移编排' },
      { key: '/dashboard/upgrade', icon: <SwapOutlined />, label: '升级管理' },
      { key: '/dashboard/arch-upgrade', icon: <SwapOutlined />, label: '架构升级' },
    ],
  },
  { key: '/dashboard/topology', icon: <ApartmentOutlined />, label: '拓扑视图' },
  {
    key: '/dashboard/diagnostics',
    icon: <ExperimentOutlined />,
    label: '智能诊断',
    children: [
      { key: '/dashboard/inspection', icon: <FileSearchOutlined />, label: '巡检' },
      { key: '/dashboard/ai-diagnosis', icon: <RobotOutlined />, label: 'AI 诊断' },
      { key: '/dashboard/fault-injection', icon: <ThunderboltOutlined />, label: '故障注入' },
      { key: '/dashboard/drills', icon: <ExperimentOutlined />, label: '演练' },
    ],
  },
  {
    key: '/dashboard/alert-center',
    icon: <AlertOutlined />,
    label: '告警中心',
    children: [
      { key: '/dashboard/alert-rules', icon: <AlertOutlined />, label: '告警规则' },
      { key: '/dashboard/alert-manage', icon: <AlertOutlined />, label: '告警管理' },
    ],
  },
  { key: '/dashboard/approvals', icon: <SafetyOutlined />, label: '审批管理' },
  {
    key: '/dashboard/security',
    icon: <LockOutlined />,
    label: '安全与合规',
    children: [
      { key: '/dashboard/audit-logs', icon: <AuditOutlined />, label: '审计日志' },
      { key: '/dashboard/audit-verify', icon: <AuditOutlined />, label: '审计链验证' },
      { key: '/dashboard/masking', icon: <LockOutlined />, label: '数据脱敏' },
      { key: '/dashboard/key-rotation', icon: <KeyOutlined />, label: '密钥轮换' },
      { key: '/dashboard/license', icon: <CopyrightOutlined />, label: '许可证' },
      { key: '/dashboard/security-settings', icon: <SettingOutlined />, label: '系统设置' },
    ],
  },
  {
    key: '/dashboard/system',
    icon: <SettingOutlined />,
    label: '系统管理',
    children: [
      { key: '/dashboard/data-storage', icon: <HddOutlined />, label: '数据存储' },
      { key: '/dashboard/agent-manage', icon: <DesktopOutlined />, label: 'Agent 管理' },
      { key: '/dashboard/plugins', icon: <AppstoreOutlined />, label: '插件管理' },
      ...(hasUserManage ? [{ key: '/dashboard/users', icon: <UserOutlined />, label: '用户与认证' }] : []),
      { key: '/dashboard/parameter-templates', icon: <FileTextOutlined />, label: '参数模板' },
      { key: '/dashboard/parameter-enhanced', icon: <FileTextOutlined />, label: '参数模板（增强）' },
    ],
  },
]

/**
 * Find the selected menu key based on current pathname.
 *
 * Exact match wins. Otherwise fall back to the longest matching prefix so that
 * `/dashboard/migration-orchestrate` is not swallowed by the sibling
 * `/dashboard/migration` key.
 */
export const findSelectedKey = (pathname: string, items: MenuItem[]): string => {
  const leafKeys: string[] = []
  const groupKeys: string[] = []
  for (const item of items) {
    if (!item || typeof item === 'number') continue
    if ('children' in item && item.children) {
      groupKeys.push(String(item.key))
      for (const child of item.children as MenuItem[]) {
        if (child && typeof child !== 'number') leafKeys.push(String(child.key))
      }
    } else {
      leafKeys.push(String(item.key))
    }
  }

  if (leafKeys.includes(pathname)) return pathname

  const prefixMatches = leafKeys
    .filter((key) => pathname.startsWith(key))
    .sort((a, b) => b.length - a.length)
  if (prefixMatches.length > 0) return prefixMatches[0]

  for (const key of groupKeys) {
    if (pathname.startsWith(key)) return key
  }
  return '/dashboard/home'
}

/** Find the open submenu keys for the current route. */
export const findOpenKeys = (pathname: string, items: MenuItem[]): string[] => {
  const exact = findSelectedKey(pathname, items)
  for (const item of items) {
    if (!item || typeof item === 'number') continue
    if ('children' in item && item.children) {
      const hit = (item.children as MenuItem[]).find(
        (child) => child && typeof child !== 'number' && String(child.key) === exact,
      )
      if (hit) return [String(item.key)]
    }
  }
  return ['/dashboard/resources']
}

/** Map menu items into antd Menu props with navigation handlers. */
export const mapMenuItemsWithNavigate = (
  items: MenuItem[],
  onNavigate: (path: string) => void,
): MenuItem[] => {
  const mapped = items.map((item) => {
    if (!item || typeof item === 'number') return item
    if ('children' in item && item.children) {
      return {
        ...(item as unknown as object),
        children: (item.children as MenuItem[]).map((child) => {
          if (!child || typeof child === 'number') return child
          return {
            ...(child as unknown as object),
            onClick: () => onNavigate(String((child as unknown as Record<string, unknown>).key)),
          }
        }),
      }
    }
    return {
      ...(item as unknown as object),
      onClick: () => onNavigate(String((item as unknown as Record<string, unknown>).key)),
    }
  })
  return mapped as MenuItem[]
}
