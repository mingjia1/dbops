import React from 'react'
import {
  AlertOutlined, ApartmentOutlined, AuditOutlined, BarChartOutlined,
  CloudOutlined, ClusterOutlined, DashboardOutlined, DatabaseOutlined, DesktopOutlined,
  FileTextOutlined, HddOutlined, HeartOutlined,
  AppstoreOutlined, RetweetOutlined, SettingOutlined, SwapOutlined, PartitionOutlined,
  SafetyOutlined, UserOutlined,
} from '@ant-design/icons'
import type { MenuProps } from 'antd'

export type MenuItem = Required<MenuProps>['items'][number]

/** Build the full sidebar menu for the dashboard shell. */
export const getDashboardMenuItems = (hasUserManage: boolean): MenuItem[] => [
  { key: '/dashboard/monitor', icon: <BarChartOutlined />, label: '监控仪表盘' },
  { key: '/dashboard/home', icon: <DashboardOutlined />, label: '总览' },
  {
    key: '/dashboard/resources',
    icon: <DesktopOutlined />,
    label: '主机与实例',
    children: [
      { key: '/dashboard/hosts', icon: <DesktopOutlined />, label: '主机管理' },
      { key: '/dashboard/instances', icon: <DatabaseOutlined />, label: '实例管理' },
    ],
  },
  { key: '/dashboard/env-check', icon: <SettingOutlined />, label: '环境检查' },
  { key: '/dashboard/backup', icon: <CloudOutlined />, label: '备份管理' },
  { key: '/dashboard/cluster-deploy', icon: <ClusterOutlined />, label: '集群部署' },
  { key: '/dashboard/ha', icon: <HeartOutlined />, label: '高可用管理' },
  { key: '/dashboard/role-switch', icon: <RetweetOutlined />, label: '角色切换' },
  { key: '/dashboard/upgrade', icon: <SwapOutlined />, label: '升级管理' },
  { key: '/dashboard/migration', icon: <PartitionOutlined />, label: '数据迁移' },
  { key: '/dashboard/topology', icon: <ApartmentOutlined />, label: '拓扑视图' },
  { key: '/dashboard/approvals', icon: <SafetyOutlined />, label: '审批管理' },
  { key: '/dashboard/audit-logs', icon: <AuditOutlined />, label: '审计日志' },
  {
    key: '/dashboard/system',
    icon: <SettingOutlined />,
    label: '系统管理',
    children: [
      { key: '/dashboard/data-storage', icon: <HddOutlined />, label: '数据存储' },
      { key: '/dashboard/agent-manage', icon: <DesktopOutlined />, label: 'Agent 管理' },
      { key: '/dashboard/plugins', icon: <AppstoreOutlined />, label: '插件管理' },
      ...(hasUserManage ? [{ key: '/dashboard/users', icon: <UserOutlined />, label: '用户与认证' }] : []),
      { key: '/dashboard/alert-rules', icon: <AlertOutlined />, label: '告警规则' },
      { key: '/dashboard/parameter-templates', icon: <FileTextOutlined />, label: '参数模板' },
      { key: '/dashboard/security-settings', icon: <SettingOutlined />, label: '系统设置' },
    ],
  },
]

/** Find the selected menu key based on current pathname. */
export const findSelectedKey = (pathname: string, items: MenuItem[]): string => {
  for (const item of items) {
    if (!item || typeof item === 'number') continue
    if ('children' in item && item.children) {
      const hit = (item.children as MenuItem[]).find(
        (child) => child && typeof child !== 'number' && pathname.startsWith(String(child.key)),
      )
      if (hit) return String(hit.key)
    }
    if (pathname.startsWith(String(item.key))) return String(item.key)
  }
  return '/dashboard/home'
}

/** Find the open submenu keys for the current route. */
export const findOpenKeys = (pathname: string, items: MenuItem[]): string[] => {
  for (const item of items) {
    if (!item || typeof item === 'number') continue
    if ('children' in item && item.children) {
      const hit = (item.children as MenuItem[]).find(
        (child) => child && typeof child !== 'number' && pathname.startsWith(String(child.key)),
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
): MenuItem[] =>
  items.map((item) => {
    if (!item || typeof item === 'number') return item
    if ('children' in item && item.children) {
      return {
        ...item,
        children: (item.children as MenuItem[]).map((child) => {
          if (!child || typeof child === 'number') return child
          return {
            ...child,
            onClick: () => onNavigate(String(child.key)),
          }
        }),
      }
    }
    return {
      ...item,
      onClick: () => onNavigate(String(item.key)),
    }
  })
