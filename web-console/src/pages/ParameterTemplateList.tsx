import React from 'react'
import { Card, Table, Button, Space, Tag } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'

interface Template {
  id: string
  name: string
  version: string
  description: string
  applyCount: number
  createdAt: string
}

const ParameterTemplateList: React.FC = () => {
  const columns: ColumnsType<Template> = [
    {
      title: '模板名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      render: (version) => <Tag color="blue">{version}</Tag>,
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
    },
    {
      title: '应用次数',
      dataIndex: 'applyCount',
      key: 'applyCount',
      render: (count) => `${count} 次`,
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
          <Button type="link" size="small">查看</Button>
          <Button type="link" size="small">编辑</Button>
          <Button type="link" size="small">应用</Button>
          <Button type="link" size="small" danger>删除</Button>
        </Space>
      ),
    },
  ]

  const data: Template[] = []

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title="参数模板"
        extra={
          <Button type="primary" icon={<PlusOutlined />}>
            创建模板
          </Button>
        }
      >
        <Table columns={columns} dataSource={data} rowKey="id" />
      </Card>
    </div>
  )
}

export default ParameterTemplateList