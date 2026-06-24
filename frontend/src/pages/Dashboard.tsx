import React, { useEffect, useState } from 'react'
import { Avatar, Dropdown, Form, Input, Layout, Menu, message, Modal, Space, Switch, Tooltip, Typography } from 'antd'
import {
  AlertOutlined, ApartmentOutlined, AuditOutlined, BarChartOutlined, BulbFilled, BulbOutlined,
  CloudOutlined, ClusterOutlined, DashboardOutlined, DatabaseOutlined, DesktopOutlined,
  FileTextOutlined, HddOutlined, HeartOutlined, LogoutOutlined, PartitionOutlined,
  AppstoreOutlined, RetweetOutlined, SafetyOutlined, SettingOutlined, SwapOutlined, UserOutlined,
} from '@ant-design/icons'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { getStoredThemeMode, type ThemeMode } from '../appTheme'
import { authApi } from '../services/api'
import './Dashboard.css'

const { Header, Content, Sider } = Layout

const Dashboard: React.FC = () => {
  const navigate = useNavigate()
  const location = useLocation()
  const [user, setUser] = useState<any>(null)
  const [manualOpenKeys, setManualOpenKeys] = useState<string[]>([])
  const [themeMode, setThemeMode] = useState<ThemeMode>(getStoredThemeMode)
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [resetOpen, setResetOpen] = useState(false)
  const [passwordForm] = Form.useForm()
  const [resetForm] = Form.useForm()

  useEffect(() => {
    try {
      const stored = localStorage.getItem('user')
      if (stored) setUser(JSON.parse(stored))
    } catch {
      // ignore invalid local storage
    }
  }, [])

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

  const submitChangePassword = async () => {
    const values = await passwordForm.validateFields()
    if (values.new_password !== values.confirm_password) {
      message.error('两次输入的密码不一致')
      return
    }
    await authApi.changePassword({
      current_password: values.current_password,
      new_password: values.new_password,
    })
    message.success('密码已修改，请重新登录')
    setPasswordOpen(false)
    passwordForm.resetFields()
    handleLogout()
  }

  const submitResetAllPasswords = async () => {
    const values = await resetForm.validateFields()
    if (values.new_password !== values.confirm_password) {
      message.error('两次输入的密码不一致')
      return
    }
    const res: any = await authApi.resetAllPasswords(values.new_password)
    message.success(`已重置 ${res?.data?.updated ?? 0} 个用户密码`)
    setResetOpen(false)
    resetForm.resetFields()
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
        { key: '/dashboard/alert-rules', icon: <AlertOutlined />, label: '告警规则' },
        { key: '/dashboard/parameter-templates', icon: <FileTextOutlined />, label: '参数模板' },
        { key: '/dashboard/security-settings', icon: <SafetyOutlined />, label: '安全设置' },
      ],
    },
  ]

  const selectedKey = (() => {
    for (const item of menuItems) {
      if ((item as any).children) {
        const hit = (item as any).children.find((child: any) => location.pathname.startsWith(child.key))
        if (hit) return hit.key
      }
      if (location.pathname.startsWith(item.key)) return item.key
    }
    return '/dashboard/home'
  })()

  const routeOpenKeys = (() => {
    for (const item of menuItems) {
      if ((item as any).children) {
        const hit = (item as any).children.find((child: any) => location.pathname.startsWith(child.key))
        if (hit) return [item.key]
      }
    }
    return ['/dashboard/resources']
  })()

  useEffect(() => {
    setManualOpenKeys(routeOpenKeys)
  }, [location.pathname])

  const userMenu = {
    items: [
      { key: 'profile', icon: <UserOutlined />, label: `${user?.username || '用户'}` },
      { key: 'change-password', label: '修改密码' },
      ...(user?.role === 'admin' ? [{ key: 'reset-all-passwords', label: '重置所有用户密码' }] : []),
      { type: 'divider' as const },
      { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', danger: true },
    ],
    onClick: ({ key }: { key: string }) => {
      if (key === 'change-password') setPasswordOpen(true)
      if (key === 'reset-all-passwords') setResetOpen(true)
      if (key === 'logout') handleLogout()
    },
  }

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header className="dashboard-header">
        <div className="header-left">
          <div className="header-logo"><DatabaseOutlined /></div>
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
            openKeys={manualOpenKeys}
            onOpenChange={(keys) => setManualOpenKeys(keys)}
            items={menuItems.map((item: any) => {
              if (item.children) {
                return {
                  key: item.key,
                  icon: item.icon,
                  label: item.label,
                  children: item.children.map((child: any) => ({
                    key: child.key,
                    icon: child.icon,
                    label: child.label,
                    onClick: () => navigate(child.key),
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
      <Modal title="修改密码" open={passwordOpen} onCancel={() => setPasswordOpen(false)} onOk={submitChangePassword} okText="确认修改" cancelText="取消" destroyOnClose>
        <Form form={passwordForm} layout="vertical" preserve={false}>
          <Form.Item label="当前密码" name="current_password" rules={[{ required: true, message: '请输入当前密码' }]}>
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Form.Item label="新密码" name="new_password" rules={[{ required: true, min: 6, message: '新密码至少 6 位' }]}>
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item label="确认新密码" name="confirm_password" rules={[{ required: true, message: '请再次输入新密码' }]}>
            <Input.Password autoComplete="new-password" />
          </Form.Item>
        </Form>
      </Modal>
      <Modal title="重置所有用户密码" open={resetOpen} onCancel={() => setResetOpen(false)} onOk={submitResetAllPasswords} okText="确认重置" cancelText="取消" destroyOnClose>
        <Form form={resetForm} layout="vertical" preserve={false} initialValues={{ new_password: '123456', confirm_password: '123456' }}>
          <Form.Item label="新密码" name="new_password" rules={[{ required: true, min: 6, message: '新密码至少 6 位' }]}>
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item label="确认新密码" name="confirm_password" rules={[{ required: true, message: '请再次输入新密码' }]}>
            <Input.Password autoComplete="new-password" />
          </Form.Item>
        </Form>
      </Modal>
    </Layout>
  )
}

export default Dashboard
