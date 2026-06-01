import React from 'react'
import { Card, Table, Button, Space, Tag, Progress } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'

interface Backup {
  id: string
  instanceName: string
  type: 'full' | 'incremental' | 'logical'
  size: string
  status: 'running' | 'completed' | 'failed'
  progress: number
  createdAt: string
}

const BackupManage: React.FC = () => {
  const columns: ColumnsType<Backup> = [
    {
      title: '实例名称',
      dataIndex: 'instanceName',
      key: 'instanceName',
    },
    {
      title: '备份类型',
      dataIndex: 'type',
      key: 'type',
      render: (type) => (
        <Tag color={type === 'full' ? 'blue' : type === 'incremental' ? 'green' : 'orange'}>
          {type === 'full' ? '全量备份' : type === 'incremental' ? '增量备份' : '逻辑备份'}
        </Tag>
      ),
    },
    {
      title: '大小',
      dataIndex: 'size',
      key: 'size',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status) => (
        <Tag color={status === 'running' ? 'processing' : status === 'completed' ? 'success' : 'error'}>
          {status === 'running' ? '进行中' : status === 'completed' ? '已完成' : '失败'}
        </Tag>
      ),
    },
    {
      title: '进度',
      dataIndex: 'progress',
      key: 'progress',
      render: (progress) => <Progress percent={progress} size="small" />,
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
    },
    {
      title: '操作',
      key: 'action',
      render: () => (
        <Space>
          <Button type="link" size="small">下载</Button>
          <Button type="link" size="small">恢复</Button>
          <Button type="link" size="small" danger>删除</Button>
        </Space>
      ),
    },
  ]

  const data: Backup[] = []

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title="备份管理"
        extra={
          <Button type="primary" icon={<PlusOutlined />}>
            创建备份
          </Button>
        }
      >
        <Table columns={columns} dataSource={data} rowKey="id" />
      </Card>
    </div>
  )
}

export default BackupManage