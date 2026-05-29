import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, message, Tag } from 'antd'
import { CheckOutlined, CloseOutlined, EyeOutlined } from '@ant-design/icons'
import { approvalApi, ApprovalRequest } from '@/services/api'

const ApprovalManage: React.FC = () => {
  const [approvals, setApprovals] = useState<ApprovalRequest[]>([])
  const [loading, setLoading] = useState(false)
  const [detailVisible, setDetailVisible] = useState(false)
  const [selectedApproval, setSelectedApproval] = useState<ApprovalRequest | null>(null)
  const [form] = Form.useForm()

  useEffect(() => {
    loadApprovals()
  }, [])

  const loadApprovals = async () => {
    setLoading(true)
    try {
      const data = await approvalApi.list() as any
      setApprovals(data)
    } catch (err) {
      message.error('加载审批列表失败')
    } finally {
      setLoading(false)
    }
  }

  const handleApprove = (approval: ApprovalRequest) => {
    Modal.confirm({
      title: '审批通过',
      content: '确定通过此审批请求吗？',
      onOk: async () => {
        await approvalApi.approve(approval.id, { comment: '审批通过' })
        message.success('审批通过')
        loadApprovals()
      },
    })
  }

  const handleReject = (approval: ApprovalRequest) => {
    form.resetFields()
    Modal.confirm({
      title: '审批拒绝',
      icon: null,
      content: (
        <Form form={form} layout="vertical">
          <Form.Item name="reason" label="拒绝原因" rules={[{ required: true }]}>
            <Input.TextArea rows={3} placeholder="请输入拒绝原因" />
          </Form.Item>
        </Form>
      ),
      onOk: async () => {
        const values = await form.validateFields()
        await approvalApi.reject(approval.id, { reason: values.reason })
        message.success('已拒绝')
        loadApprovals()
      },
    })
  }

  const handleViewDetail = (approval: ApprovalRequest) => {
    setSelectedApproval(approval)
    setDetailVisible(true)
  }

  const getStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      pending: { color: 'orange', text: '待审批' },
      approved: { color: 'green', text: '已通过' },
      rejected: { color: 'red', text: '已拒绝' },
    }
    const config = statusMap[status] || { color: 'default', text: status }
    return <Tag color={config.color}>{config.text}</Tag>
  }

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 150,
    },
    {
      title: '申请人',
      dataIndex: 'requester',
      key: 'requester',
    },
    {
      title: '操作类型',
      dataIndex: 'operation_type',
      key: 'operation_type',
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: '目标资源',
      dataIndex: 'target_resource',
      key: 'target_resource',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => getStatusTag(status),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
    },
    {
      title: '操作',
      key: 'action',
      width: 220,
      render: (_: any, record: ApprovalRequest) => (
        <Space>
          <Button size="small" icon={<EyeOutlined />} onClick={() => handleViewDetail(record)}>
            详情
          </Button>
          {record.status === 'pending' && (
            <>
              <Button size="small" type="primary" icon={<CheckOutlined />} onClick={() => handleApprove(record)}>
                通过
              </Button>
              <Button size="small" danger icon={<CloseOutlined />} onClick={() => handleReject(record)}>
                拒绝
              </Button>
            </>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Table
        columns={columns}
        dataSource={approvals}
        rowKey="id"
        loading={loading}
      />

      <Modal
        title="审批详情"
        open={detailVisible}
        onCancel={() => setDetailVisible(false)}
        footer={null}
      >
        {selectedApproval && (
          <div>
            <p><strong>申请人：</strong>{selectedApproval.requester}</p>
            <p><strong>操作类型：</strong>{selectedApproval.operation_type}</p>
            <p><strong>目标资源：</strong>{selectedApproval.target_resource}</p>
            <p><strong>状态：</strong>{getStatusTag(selectedApproval.status)}</p>
            <p><strong>描述：</strong>{selectedApproval.description}</p>
            <p><strong>创建时间：</strong>{selectedApproval.created_at}</p>
          </div>
        )}
      </Modal>
    </div>
  )
}

export default ApprovalManage