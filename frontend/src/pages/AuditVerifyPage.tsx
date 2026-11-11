import React, { useState } from 'react'
import { Card, Button, Alert, Descriptions, Tag, Typography } from 'antd'
import { message } from '../components/antdMessage'
import { SafetyOutlined, CheckCircleOutlined, CloseCircleOutlined, SearchOutlined } from '@ant-design/icons'
import api from '../services/api'

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

export default function AuditVerifyPage() {
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

  const d = result?.details

  return (
    <div style={{ padding: 24 }}>
      <Card title={<><SafetyOutlined /> 审计日志链验证</>}
        extra={<Button type="primary" icon={<SearchOutlined />} loading={verifying} onClick={handleVerify}>验证完整性</Button>}>
        <Alert type="info" message="审计链验证" description="从链头逐条重算 HMAC-SHA256 签名并校验 prev_hash 链式链接，检测任意条目是否被篡改或删除。" style={{ marginBottom: 16 }} />
        {result && (
          <Card type="inner">
            {result.valid ? (
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
                    <Typography.Text code copyable style={{ fontSize: 12 }}>{hashPreview(d?.head_hash)}</Typography.Text>
                  </Descriptions.Item>
                  <Descriptions.Item label="链尾哈希（最早条目）" span={2}>
                    <Typography.Text code copyable style={{ fontSize: 12 }}>{hashPreview(d?.tail_hash)}</Typography.Text>
                  </Descriptions.Item>
                  <Descriptions.Item label="验证时刻" span={2}>{d?.verified_at || '-'}</Descriptions.Item>
                </Descriptions>
              </>
            ) : (
              <>
                <Alert type="error" showIcon icon={<CloseCircleOutlined />}
                  message="审计链完整性验证失败" description={result.message} style={{ marginBottom: 16 }} />
                <Descriptions title="断裂详情" bordered size="small" column={2}>
                  <Descriptions.Item label="断裂条目 ID" span={2}>
                    <Typography.Text code copyable style={{ fontSize: 12 }}>{d?.broken_entry_id || '-'}</Typography.Text>
                  </Descriptions.Item>
                  <Descriptions.Item label="断裂条目时间">{d?.broken_entry_time || '-'}</Descriptions.Item>
                  <Descriptions.Item label="断裂原因"><Tag color="red">{d?.broken_reason || '-'}</Tag></Descriptions.Item>
                  {d?.claimed_prev_hash && (
                    <Descriptions.Item label="条目声称的 prev_hash" span={2}>
                      <Typography.Text code style={{ fontSize: 12 }}>{hashPreview(d.claimed_prev_hash)}</Typography.Text>
                    </Descriptions.Item>
                  )}
                  {d?.expected_prev_hash && (
                    <Descriptions.Item label="链上应有的 prev_hash" span={2}>
                      <Typography.Text code style={{ fontSize: 12 }}>{hashPreview(d.expected_prev_hash)}</Typography.Text>
                    </Descriptions.Item>
                  )}
                  {d?.stored_hash && (
                    <Descriptions.Item label="存储的 hash" span={2}>
                      <Typography.Text code style={{ fontSize: 12 }}>{hashPreview(d.stored_hash)}</Typography.Text>
                    </Descriptions.Item>
                  )}
                  {d?.recomputed_hash && (
                    <Descriptions.Item label="重算的 hash" span={2}>
                      <Typography.Text code style={{ fontSize: 12 }}>{hashPreview(d.recomputed_hash)}</Typography.Text>
                    </Descriptions.Item>
                  )}
                  <Descriptions.Item label="已校验条目数">{d?.total_entries ?? '-'}</Descriptions.Item>
                  <Descriptions.Item label="验证时刻">{d?.verified_at || '-'}</Descriptions.Item>
                </Descriptions>
              </>
            )}
          </Card>
        )}
      </Card>
    </div>
  )
}
