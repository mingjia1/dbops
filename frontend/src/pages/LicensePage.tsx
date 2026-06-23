import React, { useState, useEffect } from 'react'
import { Card, Descriptions, Button, Upload, message, Tag, Space, Alert } from 'antd'
import { SafetyCertificateOutlined, UploadOutlined, ReloadOutlined } from '@ant-design/icons'
import api from '../services/api'

interface LicenseInfo {
  tier: string
  license_key: string
  issued_to: string
  issued_at: string
  expires_at: string
  max_nodes: number
  active: boolean
}

interface Features {
  features: string[]
}

export default function LicensePage() {
  const [license, setLicense] = useState<LicenseInfo | null>(null)
  const [features, setFeatures] = useState<string[]>([])
  const [loading, setLoading] = useState(false)

  const fetchData = async () => {
    setLoading(true)
    try {
      const [licRes, featRes] = await Promise.all([api.get('/license'), api.get('/license/features')])
      setLicense(licRes.data?.data || null)
      setFeatures(featRes.data?.data?.features || [])
    } catch {}
    finally { setLoading(false) }
  }

  useEffect(() => { fetchData() }, [])

  const tierColors: Record<string, string> = { community: 'green', enterprise: 'gold', trial: 'blue' }

  return (
    <div style={{ padding: 24 }}>
      <Card title={<><SafetyCertificateOutlined /> 许可证管理</>} loading={loading}
        extra={<Button icon={<ReloadOutlined />} onClick={fetchData}>刷新</Button>}>
        {license ? (
          <>
            <Descriptions bordered size="small" column={2}>
              <Descriptions.Item label="版本"><Tag color={tierColors[license.tier] || 'default'}>{license.tier.toUpperCase()}</Tag></Descriptions.Item>
              <Descriptions.Item label="状态">{license.active ? <Tag color="green">有效</Tag> : <Tag color="red">已过期</Tag>}</Descriptions.Item>
              <Descriptions.Item label="授权对象">{license.issued_to}</Descriptions.Item>
              <Descriptions.Item label="节点上限">{license.max_nodes}</Descriptions.Item>
              <Descriptions.Item label="签发日期">{new Date(license.issued_at).toLocaleDateString()}</Descriptions.Item>
              <Descriptions.Item label="过期日期">{new Date(license.expires_at).toLocaleDateString()}</Descriptions.Item>
            </Descriptions>
            {features.length > 0 && (
              <Card type="inner" title="可用功能" size="small" style={{ marginTop: 16 }}>
                <Space wrap>{features.map((f, i) => <Tag key={i}>{f}</Tag>)}</Space>
              </Card>
            )}
          </>
        ) : (
          <Alert type="info" message="未检测到许可证" description="当前运行在社区版模式下，上传许可证文件以解锁更多功能。" />
        )}
        <Upload accept=".json,.lic" showUploadList={false} customRequest={async ({ file }) => {
          const formData = new FormData()
          formData.append('license', file)
          try { await api.post('/license/upload', formData); message.success('许可证上传成功'); fetchData() }
          catch (err: any) { message.error(`上传失败: ${err.message}`) }
        }}>
          <Button icon={<UploadOutlined />} style={{ marginTop: 16 }}>上传许可证文件</Button>
        </Upload>
      </Card>
    </div>
  )
}
