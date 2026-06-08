import React, { useEffect, useState } from 'react'
import { Layout, Menu, Dropdown, Avatar, Space, Typography, Switch, Tooltip } from 'antd'
import { BulbOutlined, BulbFilled } from '@ant-design/icons'
import { getStoredThemeMode, type ThemeMode } from '../appTheme'
import './Dashboard.css'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import {
  DashboardOutlined, DesktopOutlined, DatabaseOutlined,
  SettingOutlined, CloudOutlined, BarChartOutlined,
  AlertOutlined, SwapOutlined, PartitionOutlined,
  ApartmentOutlined, FileTextOutlined, SafetyOutlined,
  AuditOutlined, LogoutOutlined, UserOutlined,
  ClusterOutlined, HeartOutlined, RetweetOutlined, HddOutlined,
} from '@ant-design/icons'

const { Header, Content, Sider } = Layout

const Dashboard: React.FC = () => {
  const navigate = useNavigate()
  const location = useLocation()
  const [user, setUser] = useState<any>(null)

  useEffect(() => {
    try {
      const u = localStorage.getItem('user')
      if (u) setUser(JSON.parse(u))
    } catch { /* ignore */ }
  }, [])

  // P0: 之前只改 <html data-theme> + localStorage, ConfigProvider 不重渲.
  // 修: 派发 'app:theme-change' CustomEvent, ThemeRoot (main.tsx) 监听到后
  // 调 setMode + 重渲 ConfigProvider, 这样 antd Button/Table/Card 全部跟着切.
  const [themeMode, setThemeMode] = useState<ThemeMode>(getStoredThemeMode)
  const toggleTheme = (checked: boolean) => {
    const next: ThemeMode = checked ? 'dark' : 'light'
    setThemeMode(next)
    window.dispatchEvent(new CustomEvent<ThemeMode>('app:theme-change', { detail: next }))
  }

  const handleLogout = () => {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    navigate('/login', { replace: true })
  }

  const menuItems = [
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
    { key: '/dashboard/env-check', icon: <SettingOutlined />, label: '环境检测' },
    { key: '/dashboard/backup', icon: <CloudOutlined />, label: '备份管理' },
    { key: '/dashboard/cluster-deploy', icon: <ClusterOutlined />, label: '集群部署' },
    { key: '/dashboard/ha', icon: <HeartOutlined />, label: '高可用管理' },
    { key: '/dashboard/role-switch', icon: <RetweetOutlined />, label: '角色切换' },
    { key: '/dashboard/data-storage', icon: <HddOutlined />, label: '数据存储' },
    { key: '/dashboard/alert-rules', icon: <AlertOutlined />, label: '告警规则' },
    { key: '/dashboard/upgrade', icon: <SwapOutlined />, label: '升级管理' },
    { key: '/dashboard/migration', icon: <PartitionOutlined />, label: '数据迁移' },
    { key: '/dashboard/topology', icon: <ApartmentOutlined />, label: '拓扑视图' },
    { key: '/dashboard/parameter-templates', icon: <FileTextOutlined />, label: '参数模板' },
    { key: '/dashboard/approvals', icon: <SafetyOutlined />, label: '审批管理' },
    { key: '/dashboard/audit-logs', icon: <AuditOutlined />, label: '审计日志' },
  ]

  const selectedKey = (() => {
    for (const m of menuItems) {
      if ((m as any).children) {
        const hit = (m as any).children.find((c: any) => location.pathname.startsWith(c.key))
        if (hit) return hit.key
      }
      if (location.pathname.startsWith(m.key)) return m.key
    }
    return '/dashboard/home'
  })()

  // P2: 之前用 defaultOpenKeys 只在 mount 时展开一次.
  // 用户深链接到 /dashboard/hosts/:id 或 /dashboard/instances 后,
  // antd Menu 4+ 不会自动展开 parent group (需要受控 openKeys).
  // 修: 根据当前 path 反推应该展开的 parent, 用 openKeys (受控).
  const openKeys = (() => {
    for (const m of menuItems) {
      if ((m as any).children) {
        const hit = (m as any).children.find((c: any) => location.pathname.startsWith(c.key))
        if (hit) return [m.key]
      }
    }
    return ['/dashboard/resources']
  })()

  const userMenu = {
    items: [
      { key: 'profile', icon: <UserOutlined />, label: `${user?.username || '用户'}` },
      { type: 'divider' as const },
      { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', danger: true },
    ],
    onClick: ({ key }: { key: string }) => {
      if (key === 'logout') handleLogout()
    },
  }

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header className="dashboard-header">
        <div className="header-left">
          <div className="header-logo">
            <DatabaseOutlined />
          </div>
          <Typography.Title level={4} style={{ color: '#fff', margin: 0, fontWeight: 600 }}>
            MySQL 运维平台
          </Typography.Title>
        </div>
        <div className="header-right">
          <Tooltip title={themeMode === 'dark' ? '切换到亮色' : '切换到暗色'}>
            <Switch
              checked={themeMode === 'dark'}
              onChange={toggleTheme}
              checkedChildren={<BulbFilled />}
              unCheckedChildren={<BulbOutlined />}
              style={{ marginRight: 16 }}
            />
          </Tooltip>
          <Dropdown menu={userMenu} placement="bottomRight">
            <Space style={{ cursor: 'pointer', color: 'rgba(255,255,255,0.85)' }}>
              <Avatar size="small" icon={<UserOutlined />} style={{ backgroundColor: '#1890ff' }} />
              <span>{user?.username || '用户'}</span>
            </Space>
          </Dropdown>
        </div>
      </Header>
      <Layout>
        <Sider width={220} className="dashboard-sider" theme={themeMode}>
          <Menu
            mode="inline"
            theme={themeMode}
            selectedKeys={[selectedKey]}
            openKeys={openKeys}
            items={menuItems.map((item: any) => {
              if (item.children) {
                return {
                  key: item.key,
                  icon: item.icon,
                  label: item.label,
                  children: item.children.map((c: any) => ({
                    key: c.key,
                    icon: c.icon,
                    label: c.label,
                    onClick: () => navigate(c.key),
                  })),
                }
              }
              return {
                key: item.key,
                icon: item.icon,
                label: item.label,
                onClick: () => navigate(item.key),
              }
            })}
          />
        </Sider>
        <Content className="dashboard-content">
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}

export default Dashboard
