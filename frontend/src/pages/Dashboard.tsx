import React, { useEffect, useState } from 'react'
import { Avatar, Dropdown, Form, Input, Layout, Menu, message, Modal, Space, Switch, Tooltip, Typography } from 'antd'
import { BulbFilled, BulbOutlined, DatabaseOutlined, LogoutOutlined, UserOutlined } from '@ant-design/icons'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { authApi } from '../services/api'
import { useTheme } from '../hooks/useTheme'
import { getDashboardMenuItems, findSelectedKey, findOpenKeys, mapMenuItemsWithNavigate } from '../services/dashboardMenu'
import './Dashboard.css'

const { Header, Content, Sider } = Layout

const Dashboard: React.FC = () => {
  const navigate = useNavigate()
  const location = useLocation()
  const [user, setUser] = useState<any>(null)
  const [manualOpenKeys, setManualOpenKeys] = useState<string[]>([])
  const { themeMode, toggleTheme } = useTheme()
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

  const hasPermission = (permission: string) => {
    const permissions = user?.permissions || []
    return permissions.includes('*') || permissions.includes(permission)
  }

  const menuItems = getDashboardMenuItems(hasPermission('user:manage'))
  const selectedKey = findSelectedKey(location.pathname, menuItems)
  const routeOpenKeys = findOpenKeys(location.pathname, menuItems)

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
        <Sider width={220} className="dashboard-sider" theme={themeMode}>              <Menu
                mode="inline"
                theme={themeMode}
                selectedKeys={[selectedKey]}
                openKeys={manualOpenKeys}
                onOpenChange={(keys) => setManualOpenKeys(keys)}
                items={mapMenuItemsWithNavigate(menuItems, (path) => navigate(path))}
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
