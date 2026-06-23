import React, { useState, useEffect } from 'react'
import { Card, Select, Button, Steps, Alert, Spin, Space, Tag, message } from 'antd'
import { CheckCircleOutlined, WarningOutlined, LoadingOutlined } from '@ant-design/icons'
import api from '../services/api'

interface VersionEntry {
  id: string
  flavor: string
  version: string
  is_lts: boolean
  eol_date: string
}

interface CompatibilityResult {
  compatible: boolean
  version_gap: string
  warnings: string[]
  errors: string[]
  recommended_steps: string[]
}

type UpgradeStrategy = 'in-place' | 'rolling' | 'logical'

interface UpgradeWizardProps {
  instanceID: string
  currentVersion?: string
  currentFlavor?: string
  onComplete?: () => void
}

export const UpgradeWizard: React.FC<UpgradeWizardProps> = ({
  instanceID, currentVersion, currentFlavor, onComplete,
}) => {
  const [step, setStep] = useState(0)
  const [catalog, setCatalog] = useState<VersionEntry[]>([])
  const [targetVersion, setTargetVersion] = useState<string>('')
  const [strategy, setStrategy] = useState<UpgradeStrategy>('in-place')
  const [compatResult, setCompatResult] = useState<CompatibilityResult | null>(null)
  const [checking, setChecking] = useState(false)
  const [upgrading, setUpgrading] = useState(false)

  useEffect(() => {
    api.get('/versions').then(res => setCatalog(res.data?.data || [])).catch(() => {})
  }, [])

  const handlePreCheck = async () => {
    if (!targetVersion) { message.warning('请选择目标版本'); return }
    setChecking(true)
    try {
      const res = await api.post(`/instances/${instanceID}/upgrade/compatibility`, {
        target_version: targetVersion,
      })
      setCompatResult(res.data?.data || null)
      setStep(1)
    } catch (err: any) {
      message.error(`兼容性预检失败: ${err.message}`)
    } finally {
      setChecking(false)
    }
  }

  const handleExecute = async () => {
    setUpgrading(true)
    try {
      await api.post(`/instances/${instanceID}/upgrade`, {
        target_version: targetVersion,
        strategy,
      })
      message.success('升级任务已提交')
      setStep(3)
      onComplete?.()
    } catch (err: any) {
      message.error(`升级失败: ${err.message}`)
    } finally {
      setUpgrading(false)
    }
  }

  const filtered = catalog.filter(v => v.flavor === (currentFlavor || 'mysql'))

  const steps = [
    { title: '选择版本', description: '选择目标版本' },
    { title: '兼容性预检', description: '检查升级路径' },
    { title: '选择策略', description: '确认升级方式' },
    { title: '执行', description: '提交升级' },
  ]

  return (
    <Card title="版本升级向导" style={{ maxWidth: 700 }}>
      <Steps current={step} items={steps} style={{ marginBottom: 24 }} />

      {step === 0 && (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <div>
            <div style={{ marginBottom: 8 }}>当前版本: <Tag>{currentVersion || '未知'}</Tag></div>
            <Select
              style={{ width: '100%' }}
              placeholder="选择目标版本"
              value={targetVersion || undefined}
              onChange={setTargetVersion}
              options={filtered.map(v => ({
                label: `${v.flavor} ${v.version}${v.is_lts ? ' (LTS)' : ''}  —  EOL: ${v.eol_date}`,
                value: v.version,
              }))}
            />
          </div>
          <Button type="primary" onClick={handlePreCheck} loading={checking}>
            兼容性预检
          </Button>
        </Space>
      )}

      {step === 1 && compatResult && (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          {compatResult.compatible ? (
            <Alert type="success" showIcon icon={<CheckCircleOutlined />}
              message="兼容性检查通过" description={`版本跨度: ${compatResult.version_gap}`} />
          ) : (
            <Alert type="error" showIcon icon={<WarningOutlined />}
              message="不兼容" description={compatResult.errors.join('; ')} />
          )}
          {compatResult.warnings.length > 0 && (
            <Alert type="warning" showIcon message="警告"
              description={compatResult.warnings.join('; ')} />
          )}
          {compatResult.recommended_steps.length > 0 && (
            <Card type="inner" title="建议步骤" size="small">
              <ul>{compatResult.recommended_steps.map((s, i) => <li key={i}>{s}</li>)}</ul>
            </Card>
          )}
          <Space>
            <Button onClick={() => setStep(0)}>上一步</Button>
            <Button type="primary" disabled={!compatResult.compatible} onClick={() => setStep(2)}>
              选择升级策略
            </Button>
          </Space>
        </Space>
      )}

      {step === 2 && (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Select style={{ width: '100%' }} value={strategy} onChange={setStrategy} options={[
            { label: '原地升级 (In-Place) — 直接替换二进制，停机时间短', value: 'in-place' },
            { label: '滚动升级 (Rolling) — 逐节点升级，适用于集群', value: 'rolling' },
            { label: '逻辑迁移 (Logical) — 导出/导入，适用于跨大版本', value: 'logical' },
          ]} />
          <Space>
            <Button onClick={() => setStep(1)}>上一步</Button>
            <Button type="primary" onClick={handleExecute} loading={upgrading}>
              确认执行升级
            </Button>
          </Space>
        </Space>
      )}

      {step === 3 && (
        <Alert type="success" showIcon message="升级任务已提交，请在任务面板查看进度。" />
      )}
    </Card>
  )
}

export default UpgradeWizard
