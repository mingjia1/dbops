import React, { useEffect, useState } from 'react'
import { Button, Card, Descriptions, Form, Input, Modal, Select, Space, Table, Tag, message } from 'antd'
import { CheckOutlined, CloseOutlined, EyeOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { approvalApi, type ApprovalRequest } from '../services/api'

const statusColor = (status?: string) => {
  if (status === 'approved') return 'success'
  if (status === 'rejected') return 'error'
  return 'warning'
}

const currentUserId = () => {
  try {
    const raw = localStorage.getItem('user')
    if (!raw) return 'admin'
    return JSON.parse(raw)?.id || JSON.parse(raw)?.username || 'admin'
  } catch {
    return 'admin'
  }
}

const ApprovalManage: React.FC = () => {
  const [data, setData] = useState<ApprovalRequest[]>([])
  const [loading, setLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [status, setStatus] = useState<string | undefined>()
  const [viewing, setViewing] = useState<ApprovalRequest | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [form] = Form.useForm()

  const fetchData = async (nextStatus = status) => {
    setLoading(true)
    try {
      const res: any = await approvalApi.list(nextStatus)
      setData(res?.data || [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
  }, [])

  const submitCreate = async () => {
    const values = await form.validateFields()
    setSubmitting(true)
    try {
      await approvalApi.create({ requester_id: currentUserId(), ...values })
      message.success('审批申请已创建')
      setCreateOpen(false)
      fetchData()
    } finally {
      setSubmitting(false)
    }
  }

  const approve = (item: ApprovalRequest) => {
    Modal.confirm({
      title: '通过审批',
      content: `确认通过 ${item.operation_type} 申请？`,
      onOk: async () => {
        await approvalApi.approve(item.id, { comment: 'approved' })
        message.success('已通过')
        fetchData()
      },
    })
  }

  const reject = (item: ApprovalRequest) => {
    Modal.confirm({
      title: '拒绝审批',
      content: `确认拒绝 ${item.operation_type} 申请？`,
      okButtonProps: { danger: true },
      onOk: async () => {
        await approvalApi.reject(item.id, { comment: 'rejected' })
        message.success('已拒绝')
        fetchData()
      },
    })
  }

  const columns: ColumnsType<ApprovalRequest> = [
    { title: '操作类型', dataIndex: 'operation_type', key: 'operation_type', render: (v) => <Tag color="blue">{v}</Tag> },
    { title: '申请人', dataIndex: 'requester', key: 'requester' },
    { title: '资源', dataIndex: 'target_resource', key: 'target_resource' },
    { title: '原因', dataIndex: 'description', key: 'description', ellipsis: true },
    { title: '优先级', dataIndex: 'priority', key: 'priority', width: 90 },
    { title: '状态', dataIndex: 'status', key: 'status', render: (v) => <Tag color={statusColor(v)}>{v}</Tag> },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: (v) => (v ? new Date(v).toLocaleString() : '-') },
    {
      title: '操作',
      key: 'action',
      width: 260,
      render: (_, item) => (
        <Space>
          <Button size="small" icon={<EyeOutlined />} onClick={() => setViewing(item)}>详情</Button>
          <Button size="small" icon={<CheckOutlined />} disabled={item.status !== 'pending'} onClick={() => approve(item)}>通过</Button>
          <Button size="small" danger icon={<CloseOutlined />} disabled={item.status !== 'pending'} onClick={() => reject(item)}>拒绝</Button>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card
        title="审批管理"
        extra={
          <Space>
            <Select
              allowClear
              placeholder="状态"
              style={{ width: 140 }}
              value={status}
              onChange={(v) => { setStatus(v); fetchData(v) }}
              options={['pending', 'approved', 'rejected'].map((v) => ({ value: v, label: v }))}
            />
            <Button icon={<ReloadOutlined />} onClick={() => fetchData()}>刷新</Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => { form.resetFields(); setCreateOpen(true) }}>创建申请</Button>
          </Space>
        }
      >
        <Table columns={columns} dataSource={data} rowKey="id" loading={loading} />
      </Card>

      <Modal title="创建审批申请" open={createOpen} onCancel={() => setCreateOpen(false)} onOk={submitCreate} confirmLoading={submitting} width={640}>
        <Form form={form} layout="vertical" initialValues={{ priority: 1, expiry_hours: 24, operation_type: 'parameter_apply' }}>
          <Form.Item name="operation_type" label="操作类型" rules={[{ required: true }]}>
            <Select options={['parameter_apply', 'role_switch', 'backup_restore', 'upgrade', 'migration'].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="resource_type" label="资源类型" rules={[{ required: true }]}><Input placeholder="instance / cluster / backup" /></Form.Item>
          <Form.Item name="resource_id" label="资源 ID" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="request_reason" label="申请原因"><Input.TextArea rows={3} /></Form.Item>
          <Space>
            <Form.Item name="priority" label="优先级"><Input type="number" /></Form.Item>
            <Form.Item name="expiry_hours" label="有效期(小时)"><Input type="number" /></Form.Item>
          </Space>
        </Form>
      </Modal>

      <Modal title="审批详情" open={!!viewing} onCancel={() => setViewing(null)} footer={<Button onClick={() => setViewing(null)}>关闭</Button>} width={720}>
        {viewing && (
          <Descriptions bordered column={1}>
            <Descriptions.Item label="ID">{viewing.id}</Descriptions.Item>
            <Descriptions.Item label="操作类型">{viewing.operation_type}</Descriptions.Item>
            <Descriptions.Item label="资源">{viewing.target_resource}</Descriptions.Item>
            <Descriptions.Item label="状态"><Tag color={statusColor(viewing.status)}>{viewing.status}</Tag></Descriptions.Item>
            <Descriptions.Item label="申请原因">{viewing.description || '-'}</Descriptions.Item>
            <Descriptions.Item label="审批意见">{viewing.approval_comment || '-'}</Descriptions.Item>
            <Descriptions.Item label="过期时间">{viewing.expires_at ? new Date(viewing.expires_at).toLocaleString() : '-'}</Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
    </div>
  )
}

export default ApprovalManage
