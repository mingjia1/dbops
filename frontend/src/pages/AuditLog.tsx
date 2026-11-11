import React, { useEffect, useState } from 'react'
import { Alert, Button, Card, Descriptions, Input, Modal, Space, Table, Tag, Typography } from 'antd'
import { EyeOutlined, SafetyCertificateOutlined, ReloadOutlined, SearchOutlined, CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { message } from '../components/antdMessage'
import { auditApi, type AuditLog as AuditLogType } from '../services/api'
import api from '../services/api'

const { Text } = Typography

interface ChainDetails {
  total_entries?: number
  algorithm?: string
  verified_at?: string
  oldest_entry_id?: string
  oldest_entry_time?: string
  oldest_action?: string
  newest_entry_id?: string
  newest_entry_time?: string
  newest_action?: string
  head_hash?: string
  tail_hash?: string
  broken_entry_id?: string
  broken_entry_time?: string
  broken_reason?: string
  expected_prev_hash?: string
  claimed_prev_hash?: string
  stored_hash?: string
  recomputed_hash?: string
}

interface ChainResult {
  valid: boolean
  message: string
  details?: ChainDetails
}

const hashPreview = (h?: string) => (h ? `${h.slice(0, 16)}…${h.slice(-8)}` : '-')

const ChainVerifyModal: React.FC<{ open: boolean; onClose: () => void }> = ({ open, onClose }) => {
  const [verifying, setVerifying] = useState(false)
  const [result, setResult] = useState<ChainResult | null>(null)

  const handleVerify = async () => {
    setVerifying(true)
    try {
      const res = await api.get('/audit-logs/verify-chain')
      setResult(res.data || null)
    } catch (err: any) {
      message.error(`验证失败: ${err.message}`)
    } finally { setVerifying(false) }
  }

  useEffect(() => {
    if (open) { setResult(null); handleVerify() }
  }, [open])

  const d = result?.details

  return (
    <Modal
      title={<><SafetyCertificateOutlined /> 审计链防篡改验证</>}
      open={open}
      onCancel={onClose}
      width={760}
      footer={
        <Space>
          <Button icon={<ReloadOutlined />} loading={verifying} onClick={handleVerify}>重新验证</Button>
          <Button onClick={onClose}>关闭</Button>
        </Space>
      }
    >
      <Alert
        type="info"
        showIcon
        message="从链头逐条重算 HMAC-SHA256 签名并校验 prev_hash 链式链接，检测任意条目是否被篡改、删除或插入。"
        style={{ marginBottom: 16 }}
      />
      {verifying && !result ? (
        <div style={{ textAlign: 'center', padding: 24 }}>正在校验…</div>
      ) : result && result.valid ? (
        <>
          <Alert type="success" showIcon icon={<CheckCircleOutlined />}
            message="审计链完整性验证通过" description={result.message} style={{ marginBottom: 16 }} />
          <Descriptions title="校验详情" bordered size="small" column={2}>
            <Descriptions.Item label="校验条目数">{d?.total_entries ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="签名算法"><Tag color="blue">{d?.algorithm || '-'}</Tag></Descriptions.Item>
            <Descriptions.Item label="链起始时间">{d?.oldest_entry_time || '-'}</Descriptions.Item>
            <Descriptions.Item label="链结束时间">{d?.newest_entry_time || '-'}</Descriptions.Item>
            <Descriptions.Item label="最早条目操作">{d?.oldest_action || '-'}</Descriptions.Item>
            <Descriptions.Item label="最新条目操作">{d?.newest_action || '-'}</Descriptions.Item>
            <Descriptions.Item label="链头哈希（最新条目）" span={2}>
              <Text code copyable style={{ fontSize: 12 }}>{hashPreview(d?.head_hash)}</Text>
            </Descriptions.Item>
            <Descriptions.Item label="链尾哈希（最早条目）" span={2}>
              <Text code copyable style={{ fontSize: 12 }}>{hashPreview(d?.tail_hash)}</Text>
            </Descriptions.Item>
            <Descriptions.Item label="验证时刻" span={2}>{d?.verified_at || '-'}</Descriptions.Item>
          </Descriptions>
        </>
      ) : result ? (
        <>
          <Alert type="error" showIcon icon={<CloseCircleOutlined />}
            message="审计链完整性验证失败" description={result.message} style={{ marginBottom: 16 }} />
          <Descriptions title="断裂详情" bordered size="small" column={2}>
            <Descriptions.Item label="断裂条目 ID" span={2}>
              <Text code copyable style={{ fontSize: 12 }}>{d?.broken_entry_id || '-'}</Text>
            </Descriptions.Item>
            <Descriptions.Item label="断裂条目时间">{d?.broken_entry_time || '-'}</Descriptions.Item>
            <Descriptions.Item label="断裂原因"><Tag color="red">{d?.broken_reason || '-'}</Tag></Descriptions.Item>
            {d?.claimed_prev_hash && (
              <Descriptions.Item label="条目声称的 prev_hash" span={2}>
                <Text code style={{ fontSize: 12 }}>{hashPreview(d.claimed_prev_hash)}</Text>
              </Descriptions.Item>
            )}
            {d?.expected_prev_hash && (
              <Descriptions.Item label="链上应有的 prev_hash" span={2}>
                <Text code style={{ fontSize: 12 }}>{hashPreview(d.expected_prev_hash)}</Text>
              </Descriptions.Item>
            )}
            {d?.stored_hash && (
              <Descriptions.Item label="存储的 hash" span={2}>
                <Text code style={{ fontSize: 12 }}>{hashPreview(d.stored_hash)}</Text>
              </Descriptions.Item>
            )}
            {d?.recomputed_hash && (
              <Descriptions.Item label="重算的 hash" span={2}>
                <Text code style={{ fontSize: 12 }}>{hashPreview(d.recomputed_hash)}</Text>
              </Descriptions.Item>
            )}
            <Descriptions.Item label="已校验条目数">{d?.total_entries ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="验证时刻">{d?.verified_at || '-'}</Descriptions.Item>
          </Descriptions>
        </>
      ) : null}
    </Modal>
  )
}

const AuditLog: React.FC = () => {
  const [data, setData] = useState<AuditLogType[]>([])
  const [loading, setLoading] = useState(false)
  const [viewing, setViewing] = useState<AuditLogType | null>(null)
  const [verifyOpen, setVerifyOpen] = useState(false)
  const [filters, setFilters] = useState<{ user?: string; action?: string }>({})

  const fetchData = async (nextFilters = filters) => {
    setLoading(true)
    try {
      const res: any = await auditApi.list(nextFilters)
      // 后端返回 { limit, logs: [...], total }, 提取 logs 数组
      setData(res?.data?.logs || (Array.isArray(res?.data) ? res.data : []))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
  }, [])

  const columns: ColumnsType<AuditLogType> = [
    { title: '用户', dataIndex: 'user_id', key: 'user_id', ellipsis: true, render: (v) => v || '-' },
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
            <Button type="primary" ghost icon={<SafetyCertificateOutlined />} onClick={() => setVerifyOpen(true)}>防篡改验证</Button>
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

      <ChainVerifyModal open={verifyOpen} onClose={() => setVerifyOpen(false)} />

      <Modal title="审计详情" open={!!viewing} onCancel={() => setViewing(null)} footer={<Button onClick={() => setViewing(null)}>关闭</Button>} width={760}>
        {viewing && (
          <Descriptions bordered column={1}>
            <Descriptions.Item label="ID">{viewing.id}</Descriptions.Item>
            <Descriptions.Item label="用户">{viewing.user_id}</Descriptions.Item>
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
