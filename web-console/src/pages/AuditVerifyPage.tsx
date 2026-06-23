import React, { useState } from 'react'
import { Card, Button, Table, message, Alert, Tag, Space, Input, Descriptions } from 'antd'
import { SafetyOutlined, CheckCircleOutlined, CloseCircleOutlined, SearchOutlined } from '@ant-design/icons'
import api from '../services/api'

interface ChainResult {
  valid: boolean
  message: string
  broken_at?: string
}

export default function AuditVerifyPage() {
  const [verifying, setVerifying] = useState(false)
  const [result, setResult] = useState<ChainResult | null>(null)

  const handleVerify = async () => {
    setVerifying(true)
    try {
      const res = await api.get('/audit-logs/verify-chain')
      setResult(res.data?.data || null)
    } catch (err: any) {
      message.error(`验证失败: ${err.message}`)
    } finally { setVerifying(false) }
  }

  return (
    <div style={{ padding: 24 }}>
      <Card title={<><SafetyOutlined /> 审计日志链验证</>}
        extra={<Button type="primary" icon={<SearchOutlined />} loading={verifying} onClick={handleVerify}>验证完整性</Button>}>
        <Alert type="info" message="审计链验证" description="验证所有审计日志的哈希链是否完整，检测是否有篡改。" style={{ marginBottom: 16 }} />
        {result && (
          <Card type="inner">
            {result.valid ? (
              <Alert type="success" showIcon icon={<CheckCircleOutlined />}
                message="审计链完整性验证通过" description={result.message} />
            ) : (
              <Alert type="error" showIcon icon={<CloseCircleOutlined />}
                message="审计链完整性验证失败"
                description={`链在 ${result.broken_at || '未知位置'} 处断裂。${result.message}`} />
            )}
          </Card>
        )}
      </Card>
    </div>
  )
}
