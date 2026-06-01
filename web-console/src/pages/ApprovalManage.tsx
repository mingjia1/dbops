import React from 'react'
import { Card, Table, Button, Space, Tag } from 'antd'
import type { ColumnsType } from 'antd/es/table'

interface Approval {
  id: string
  type: string
  applicant: string
  content: string
  status: 'pending' | 'approved' | 'rejected'
  createdAt: string
}

const ApprovalManage: React.FC = () => {
  const columns: ColumnsType<Approval> = [
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      render: (type) => <Tag color="blue">{type}</Tag>,
    },
    {
      title: '申请人',
      dataIndex: 'applicant',
      key: 'applicant',
    },
    {
      title: '内容',
      dataIndex: 'content',
      key: 'content',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status) => (
        <Tag color={status === 'pending' ? 'warning' : status === 'approved' ? 'success' : 'error'}>
          {status === 'pending' ? '待审批' : status === 'approved' ? '已通过' : '已拒绝'}
        </Tag>
      ),
    },
    {
      title: '申请时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
    },
    {
      title: '操作',
      key: 'action',
      render: () => (
        <Space>
          <Button type="link" size="small">审批</Button>
          <Button type="link" size="small">详情</Button>
        </Space>
      ),
    },
  ]

  const data: Approval[] = []

  return (
    <div style={{ padding: '24px' }}>
      <Card title="审批管理">
        <Table columns={columns} dataSource={data} rowKey="id" />
      </Card>
    </div>
  )
}

export default ApprovalManage