import React, { useEffect, useState } from 'react'
import { Layout, Menu, Dropdown, Avatar, Space, Typography } from 'antd'
import './Dashboard.css'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import {
  DashboardOutlined, DesktopOutlined, DatabaseOutlined,
  SettingOutlined, CloudOutlined, BarChartOutlined,
  AlertOutlined, SwapOutlined, PartitionOutlined,
  ApartmentOutlined, FileTextOutlined, SafetyOutlined,
  AuditOutlined, LogoutOutlined, UserOutlined,
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

  const handleLogout = () => {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    navigate('/login', { replace: true })
  }

  const menuItems = [
    { key: '/dashboard/home', icon: <DashboardOutlined />, label: '总览' },
    { key: '/dashboard/hosts', icon: <DesktopOutlined />, label: '主机管理' },
    { key: '/dashboard/instances', icon: <DatabaseOutlined />, label: '实例管理' },
    { key: '/dashboard/env-check', icon: <SettingOutlined />, label: '环境检测' },
    { key: '/dashboard/backup', icon: <CloudOutlined />, label: '备份管理' },
    { key: '/dashboard/monitor', icon: <BarChartOutlined />, label: '监控仪表盘' },
    { key: '/dashboard/alert-rules', icon: <AlertOutlined />, label: '告警规则' },
    { key: '/dashboard/upgrade', icon: <SwapOutlined />, label: '升级管理' },
    { key: '/dashboard/migration', icon: <PartitionOutlined />, label: '数据迁移' },
    { key: '/dashboard/topology', icon: <ApartmentOutlined />, label: '拓扑视图' },
    { key: '/dashboard/parameter-templates', icon: <FileTextOutlined />, label: '参数模板' },
    { key: '/dashboard/approvals', icon: <SafetyOutlined />, label: '审批管理' },
    { key: '/dashboard/audit-logs', icon: <AuditOutlined />, label: '审计日志' },
  ]

  const selectedKey = menuItems.find(m => location.pathname.startsWith(m.key))?.key || '/dashboard/home'

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
          <Dropdown menu={userMenu} placement="bottomRight">
            <Space style={{ cursor: 'pointer', color: 'rgba(255,255,255,0.85)' }}>
              <Avatar size="small" icon={<UserOutlined />} style={{ backgroundColor: '#1890ff' }} />
              <span>{user?.username || '用户'}</span>
            </Space>
          </Dropdown>
        </div>
      </Header>
      <Layout>
        <Sider width={220} className="dashboard-sider">
          <Menu
            mode="inline"
            selectedKeys={[selectedKey]}
            defaultOpenKeys={[]}
            items={menuItems.map(item => ({
              key: item.key,
              icon: item.icon,
              label: item.label,
              onClick: () => navigate(item.key),
            }))}
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
