import React from 'react'
import { Card, Table, Tag, Progress } from 'antd'
import type { ColumnsType } from 'antd/es/table'

interface CheckItem {
  id: string
  category: string
  item: string
  status: 'pass' | 'warning' | 'fail'
  score: number
  description: string
}

const EnvironmentCheck: React.FC = () => {
  const columns: ColumnsType<CheckItem> = [
    {
      title: '类别',
      dataIndex: 'category',
      key: 'category',
      width: 120,
    },
    {
      title: '检查项',
      dataIndex: 'item',
      key: 'item',
      width: 200,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status) => (
        <Tag color={status === 'pass' ? 'success' : status === 'warning' ? 'warning' : 'error'}>
          {status === 'pass' ? '通过' : status === 'warning' ? '警告' : '失败'}
        </Tag>
      ),
    },
    {
      title: '评分',
      dataIndex: 'score',
      key: 'score',
      width: 120,
      render: (score) => <Progress percent={score} size="small" />,
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
    },
  ]

  const data: CheckItem[] = []

  return (
    <div style={{ padding: '24px' }}>
      <Card title="环境巡检">
        <Table columns={columns} dataSource={data} rowKey="id" />
      </Card>
    </div>
  )
}

export default EnvironmentCheck