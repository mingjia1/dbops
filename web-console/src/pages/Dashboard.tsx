import React from 'react'
import { Layout, Menu, Typography } from 'antd'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import { DatabaseOutlined, SettingOutlined, CloudOutlined, BarChartOutlined } from '@ant-design/icons'

const { Header, Content, Sider } = Layout

const Dashboard: React.FC = () => {
  const navigate = useNavigate()
  const location = useLocation()

  const menuItems = [
    { key: '/instances', icon: <DatabaseOutlined />, label: '实例管理' },
    { key: '/env-check', icon: <SettingOutlined />, label: '环境检测' },
    { key: '/backup', icon: <CloudOutlined />, label: '备份管理' },
    { key: '/monitor', icon: <BarChartOutlined />, label: '监控仪表盘' },
  ]

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ background: '#001529', padding: '0 24px', display: 'flex', alignItems: 'center' }}>
        <Typography.Title level={3} style={{ color: '#fff', margin: 0 }}>
          MySQL 运维平台
        </Typography.Title>
      </Header>
      <Layout>
        <Sider width={200} style={{ background: '#fff' }}>
          <Menu
            mode="inline"
            selectedKeys={[location.pathname]}
            style={{ height: '100%', borderRight: 0 }}
            items={menuItems.map(item => ({
              key: item.key,
              icon: item.icon,
              label: item.label,
              onClick: () => navigate(item.key),
            }))}
          />
        </Sider>
        <Content style={{ padding: '24px', background: '#f0f2f5', minHeight: 280 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}

export default Dashboard