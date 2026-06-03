import React, { useEffect, useState } from 'react'
import { Card, Table, Button, Space, Tag, message, Modal } from 'antd'
import { CheckOutlined, CloseOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { approvalApi, type ApprovalRequest } from '../services/api'

const ApprovalManage: React.FC = () => {
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)

  const fetchData = () => {
    setLoading(true)
    approvalApi.list().then((res: any) => {
      setData(res?.data || [])
    }).catch(() => {}).finally(() => setLoading(false))
  }

  useEffect(() => { fetchData() }, [])

  const handleApprove = (id: string) => {
    Modal.confirm({
      title: '确认审批',
      content: '确定要通过此申请吗？',
      onOk: () => approvalApi.approve(id, {}).then(() => {
        message.success('已审批通过')
        fetchData()
      }).catch(() => {}),
    })
  }

  const handleReject = (id: string) => {
    Modal.confirm({
      title: '确认拒绝',
      content: '确定要拒绝此申请吗？',
      onOk: () => approvalApi.reject(id, { reason: '管理员拒绝' }).then(() => {
        message.success('已拒绝')
        fetchData()
      }).catch(() => {}),
    })
  }

  const columns: ColumnsType<ApprovalRequest> = [
    {
      title: '操作类型',
      dataIndex: 'operation_type',
      key: 'operation_type',
      render: (t) => <Tag color="blue">{t || '-'}</Tag>,
    },
    {
      title: '申请人',
      dataIndex: 'requester',
      key: 'requester',
      render: (r) => r || '-',
    },
    {
      title: '资源',
      dataIndex: 'target_resource',
      key: 'target_resource',
      render: (r) => r || '-',
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
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
      dataIndex: 'created_at',
      key: 'created_at',
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Space>
          <Button type="link" size="small" icon={<CheckOutlined />} onClick={() => handleApprove(record.id)}>通过</Button>
          <Button type="link" size="small" danger icon={<CloseOutlined />} onClick={() => handleReject(record.id)}>拒绝</Button>
          <Button type="link" size="small">详情</Button>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <Card title="审批管理">
        <Table columns={columns} dataSource={data} rowKey="id" loading={loading} />
      </Card>
    </div>
  )
}

export default ApprovalManage