import React, { useEffect, useState } from 'react'
import { Card, Table, Tag, Tabs, Typography } from 'antd'
import { ApiOutlined, AppstoreOutlined, CloudServerOutlined, DeploymentUnitOutlined } from '@ant-design/icons'
import api from '../services/api'

interface PluginInfo {
  name: string
  type: string
  version: string
}

const typeColors: Record<string, string> = {
  kernel: 'blue',
  arch: 'green',
  middleware: 'orange',
}

const typeLabels: Record<string, string> = {
  kernel: '内核插件',
  arch: '架构插件',
  middleware: '中间件插件',
}

const typeIcons: Record<string, React.ReactNode> = {
  kernel: <CloudServerOutlined />,
  arch: <DeploymentUnitOutlined />,
  middleware: <ApiOutlined />,
}

const pluginDescriptions: Record<string, string> = {
  'mysql-core': 'MySQL 单机部署、配置渲染、Systemd 托管与实例启停',
  'percona-core': 'Percona Server 单机部署（基于 MySQL Core）',
  'mariadb-core': 'MariaDB 单机部署，自动适配 gtid_domain_id',
  'replica-addon': '主从异步复制搭建',
  'mha-addon': 'MHA 高可用集群搭建（Manager + Primary + Replicas）',
  'mgr-addon': 'MySQL Group Replication 集群搭建',
  'pxc-galera-addon': 'Percona XtraDB Cluster / Galera 集群搭建',
  'keepalived-addon': 'Keepalived VIP 漂移中间件部署',
  'proxysql-addon': 'ProxySQL 读写分离负载均衡部署',
}

const PluginManage: React.FC = () => {
  const [plugins, setPlugins] = useState<PluginInfo[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchPlugins = async () => {
      try {
        setLoading(true)
        const res: any = await api.get('/plugins')
        setPlugins(res?.data || [])
      } catch (err: any) {
        console.error('Failed to load plugins:', err)
      } finally {
        setLoading(false)
      }
    }
    fetchPlugins()
  }, [])

  const columns = [
    {
      title: '插件名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => (
        <Typography.Text strong>{name}</Typography.Text>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      render: (type: string) => (
        <Tag color={typeColors[type] || 'default'} icon={typeIcons[type]}>
          {typeLabels[type] || type}
        </Tag>
      ),
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      render: (ver: string) => <Tag>{ver}</Tag>,
    },
    {
      title: '说明',
      key: 'description',
      render: (_: any, record: PluginInfo) => (
        <Typography.Text type="secondary">
          {pluginDescriptions[record.name] || '-'}
        </Typography.Text>
      ),
    },
    {
      title: '状态',
      key: 'status',
      render: () => <Tag color="success">已注册</Tag>,
    },
  ]

  const filterByType = (type: string) => plugins.filter((p) => p.type === type)

  const tabItems = [
    {
      key: 'all',
      label: (
        <span><AppstoreOutlined /> 全部 ({plugins.length})</span>
      ),
      children: (
        <Table
          columns={columns}
          dataSource={plugins}
          rowKey="name"
          loading={loading}
          pagination={false}
          size="middle"
        />
      ),
    },
    {
      key: 'kernel',
      label: (
        <span><CloudServerOutlined /> 内核插件 ({filterByType('kernel').length})</span>
      ),
      children: (
        <Table
          columns={columns}
          dataSource={filterByType('kernel')}
          rowKey="name"
          loading={loading}
          pagination={false}
          size="middle"
        />
      ),
    },
    {
      key: 'arch',
      label: (
        <span><DeploymentUnitOutlined /> 架构插件 ({filterByType('arch').length})</span>
      ),
      children: (
        <Table
          columns={columns}
          dataSource={filterByType('arch')}
          rowKey="name"
          loading={loading}
          pagination={false}
          size="middle"
        />
      ),
    },
    {
      key: 'middleware',
      label: (
        <span><ApiOutlined /> 中间件插件 ({filterByType('middleware').length})</span>
      ),
      children: (
        <Table
          columns={columns}
          dataSource={filterByType('middleware')}
          rowKey="name"
          loading={loading}
          pagination={false}
          size="middle"
        />
      ),
    },
  ]

  return (
    <Card title="插件管理" style={{ margin: 16 }}>
      <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
        平台内置插件一览。内核插件负责单机实例部署，架构插件负责集群化装配，中间件插件提供 VIP 漂移和读写分离能力。
      </Typography.Paragraph>
      <Tabs items={tabItems} />
    </Card>
  )
}

export default PluginManage
