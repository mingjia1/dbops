import React from 'react'
import { Layout, Typography } from 'antd'

const { Header, Content, Sider } = Layout

const Dashboard: React.FC = () => {
  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ background: '#001529', padding: '0 24px' }}>
        <Typography.Title level={3} style={{ color: '#fff', margin: '16px 0' }}>
          MySQL 运维平台
        </Typography.Title>
      </Header>
      <Layout>
        <Sider width={200} style={{ background: '#fff' }}>
        </Sider>
        <Content style={{ padding: '24px', background: '#f0f2f5' }}>
          <Typography.Title level={4}>仪表盘</Typography.Title>
        </Content>
      </Layout>
    </Layout>
  )
}

export default Dashboard