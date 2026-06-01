import React from 'react'
import { Card, Table, Button, Space, Tag } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'

interface Instance {
  id: string
  name: string
  host: string
  port: number
  version: string
  status: 'running' | 'stopped' | 'error'
  type: 'mysql' | 'mariadb' | 'percona'
}

const InstanceList: React.FC = () => {
  const columns: ColumnsType<Instance> = [
    {
      title: '实例名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '地址',
      key: 'address',
      render: (_, record) => `${record.host}:${record.port}`,
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      render: (type) => (
        <Tag color={type === 'mysql' ? 'blue' : type === 'mariadb' ? 'green' : 'orange'}>
          {type.toUpperCase()}
        </Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status) => (
        <Tag color={status === 'running' ? 'success' : status === 'stopped' ? 'default' : 'error'}>
          {status === 'running' ? '运行中' : status === 'stopped' ? '已停止' : '异常'}
        </Tag>
      ),
    },
    {
      title: '操作',
      key: 'action',
      render: () => (
        <Space>
          <Button type="link" size="small">详情</Button>
          <Button type="link" size="small">监控</Button>
          <Button type="link" size="small" danger>删除</Button>
        </Space>
      ),
    },
  ]

  const data: Instance[] = []

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title="实例列表"
        extra={
          <Button type="primary" icon={<PlusOutlined />}>
            添加实例
          </Button>
        }
      >
        <Table columns={columns} dataSource={data} rowKey="id" />
      </Card>
    </div>
  )
}

export default InstanceList