import React, { useEffect, useState } from 'react'
import { Button, Card, Descriptions, Input, Modal, Space, Table, Tag } from 'antd'
import { EyeOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { auditApi, type AuditLog as AuditLogType } from '../services/api'

const AuditLog: React.FC = () => {
  const [data, setData] = useState<AuditLogType[]>([])
  const [loading, setLoading] = useState(false)
  const [viewing, setViewing] = useState<AuditLogType | null>(null)
  const [filters, setFilters] = useState<{ user?: string; action?: string }>({})

  const fetchData = async (nextFilters = filters) => {
    setLoading(true)
    try {
      const res: any = await auditApi.list(nextFilters)
      setData(res?.data || [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
  }, [])

  const columns: ColumnsType<AuditLogType> = [
    { title: '用户', dataIndex: 'user', key: 'user', render: (v) => v || '-' },
    { title: '操作', dataIndex: 'action', key: 'action', render: (v, item) => item.operation || v || '-' },
    { title: '资源类型', dataIndex: 'resource_type', key: 'resource_type', render: (v) => v || '-' },
    { title: '资源 ID', dataIndex: 'resource_id', key: 'resource_id', ellipsis: true },
    {
      title: '结果',
      dataIndex: 'result',
      key: 'result',
      render: (v) => <Tag color={v === 'success' || !v ? 'success' : 'error'}>{v || 'success'}</Tag>,
    },
    { title: 'IP', dataIndex: 'ip_address', key: 'ip_address', render: (v) => v || '-' },
    { title: '时间', dataIndex: 'created_at', key: 'created_at', render: (v) => (v ? new Date(v).toLocaleString() : '-') },
    {
      title: '操作',
      key: 'action_view',
      width: 90,
      render: (_, item) => <Button size="small" icon={<EyeOutlined />} onClick={() => setViewing(item)}>详情</Button>,
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card
        title="审计日志"
        extra={
          <Space>
            <Input
              placeholder="用户 ID"
              prefix={<SearchOutlined />}
              allowClear
              style={{ width: 180 }}
              value={filters.user}
              onChange={(e) => setFilters({ ...filters, user: e.target.value })}
            />
            <Input
              placeholder="操作"
              allowClear
              style={{ width: 180 }}
              value={filters.action}
              onChange={(e) => setFilters({ ...filters, action: e.target.value })}
            />
            <Button type="primary" icon={<SearchOutlined />} onClick={() => fetchData()}>搜索</Button>
            <Button icon={<ReloadOutlined />} onClick={() => { const empty = {}; setFilters(empty); fetchData(empty) }}>重置</Button>
          </Space>
        }
      >
        <Table columns={columns} dataSource={data} rowKey="id" loading={loading} />
      </Card>

      <Modal title="审计详情" open={!!viewing} onCancel={() => setViewing(null)} footer={<Button onClick={() => setViewing(null)}>关闭</Button>} width={760}>
        {viewing && (
          <Descriptions bordered column={1}>
            <Descriptions.Item label="ID">{viewing.id}</Descriptions.Item>
            <Descriptions.Item label="用户">{viewing.user}</Descriptions.Item>
            <Descriptions.Item label="操作">{viewing.operation || viewing.action}</Descriptions.Item>
            <Descriptions.Item label="资源">{viewing.resource_type}:{viewing.resource_id}</Descriptions.Item>
            <Descriptions.Item label="结果">{viewing.result || 'success'}</Descriptions.Item>
            <Descriptions.Item label="错误">{viewing.error_msg || '-'}</Descriptions.Item>
            <Descriptions.Item label="详情">
              <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{viewing.details || '-'}</pre>
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
    </div>
  )
}

export default AuditLog
