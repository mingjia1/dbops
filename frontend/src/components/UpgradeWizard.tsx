import React, { useEffect, useState } from 'react'
import { Alert, Button, Card, Select, Space, Steps, Tag, message } from 'antd'
import { CheckCircleOutlined, WarningOutlined } from '@ant-design/icons'
import { upgradeApi, versionApi } from '../services/api'

interface VersionEntry {
  id: string
  flavor: string
  version: string
  is_lts: boolean
  eol_date: string
  local_available?: boolean
}

interface CompatibilityResult {
  is_compatible: boolean
  source_version: string
  target_version: string
  warning_count: number
  error_count: number
  incompatibilities: Array<{ level: string; description: string }>
  recommendations: string[]
}

type UpgradeStrategy = 'inplace' | 'rolling' | 'logical'

interface UpgradeWizardProps {
  instanceID: string
  clusterID?: string
  currentVersion?: string
  currentFlavor?: string
  onComplete?: () => void
}

export const UpgradeWizard: React.FC<UpgradeWizardProps> = ({
  instanceID, clusterID, currentVersion, currentFlavor, onComplete,
}) => {
  const [step, setStep] = useState(0)
  const [catalog, setCatalog] = useState<VersionEntry[]>([])
  const [targetVersion, setTargetVersion] = useState<string>('')
  const [strategy, setStrategy] = useState<UpgradeStrategy>('inplace')
  const [compatResult, setCompatResult] = useState<CompatibilityResult | null>(null)
  const [planID, setPlanID] = useState<string>('')
  const [checking, setChecking] = useState(false)
  const [upgrading, setUpgrading] = useState(false)

  useEffect(() => {
    versionApi.list()
      .then((res: any) => setCatalog(res?.data || []))
      .catch((err: any) => console.warn('Failed to load version catalog:', err))
  }, [])

  const handlePreCheck = async () => {
    if (!targetVersion) {
      message.warning('Please select a target version')
      return
    }
    setChecking(true)
    try {
      const [compat, plan] = await Promise.all([
        upgradeApi.checkCompat({
          instance_id: instanceID,
          target_version: targetVersion,
          target_flavor: currentFlavor || 'mysql',
        }),
        upgradeApi.planPath({
          instance_id: instanceID,
          target_version: targetVersion,
          target_flavor: currentFlavor || 'mysql',
          strategy,
          source_version: currentVersion,
        }),
      ])
      setCompatResult(compat?.data || null)
      setPlanID(plan?.data?.plan_id || '')
      setStep(1)
    } catch (err: any) {
      message.error(`Compatibility check failed: ${err.message}`)
    } finally {
      setChecking(false)
    }
  }

  const handleExecute = async () => {
    if (!planID) {
      message.error('Upgrade plan is missing. Run pre-check again.')
      return
    }
    setUpgrading(true)
    try {
      if (strategy === 'rolling' && !clusterID) {
        message.error('Rolling upgrade requires a cluster ID')
        return
      }
      const payload = {
        instance_id: instanceID,
        plan_id: planID,
        target_version: targetVersion,
        target_flavor: currentFlavor || 'mysql',
        backup_enabled: true,
      }
      if (strategy === 'logical') {
        await upgradeApi.executeLogical({ ...payload, parallelism: 4, batch_size: 1000 })
      } else if (strategy === 'rolling') {
        await upgradeApi.executeRolling({
          cluster_id: clusterID,
          plan_id: planID,
          target_version: targetVersion,
          max_in_parallel: 1,
          health_check_interval: 30,
        })
      } else {
        await upgradeApi.executeInPlace(payload)
      }
      message.success('Upgrade task submitted')
      setStep(3)
      onComplete?.()
    } catch (err: any) {
      message.error(`Upgrade failed: ${err.message}`)
    } finally {
      setUpgrading(false)
    }
  }

  const filtered = catalog.filter((v) => v.flavor === (currentFlavor || 'mysql'))

  const steps = [
    { title: 'Version', description: 'Select target version' },
    { title: 'Check', description: 'Run compatibility check' },
    { title: 'Strategy', description: 'Choose execution mode' },
    { title: 'Submit', description: 'Start the upgrade task' },
  ]

  const warnings = compatResult?.incompatibilities.filter((item) => item.level !== 'error').map((item) => item.description) || []
  const errors = compatResult?.incompatibilities.filter((item) => item.level === 'error').map((item) => item.description) || []

  return (
    <Card title="Upgrade Wizard" style={{ maxWidth: 700 }}>
      <Steps current={step} items={steps} style={{ marginBottom: 24 }} />

      {step === 0 && (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <div>
            <div style={{ marginBottom: 8 }}>Current version: <Tag>{currentVersion || 'unknown'}</Tag></div>
            <Select
              style={{ width: '100%' }}
              placeholder="Select target version"
              value={targetVersion || undefined}
              onChange={setTargetVersion}
              options={filtered.map((v) => ({
                label: `${v.flavor} ${v.version}${v.is_lts ? ' (LTS)' : ''}${v.local_available ? ' [Exists]' : ' [Download]'} - EOL: ${v.eol_date}`,
                value: v.version,
              }))}
            />
          </div>
          <Button type="primary" onClick={handlePreCheck} loading={checking}>
            Run compatibility check
          </Button>
        </Space>
      )}

      {step === 1 && compatResult && (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          {compatResult.is_compatible ? (
            <Alert
              type="success"
              showIcon
              icon={<CheckCircleOutlined />}
              message="Compatibility check passed"
              description={`Source ${compatResult.source_version} -> target ${compatResult.target_version}`}
            />
          ) : (
            <Alert
              type="error"
              showIcon
              icon={<WarningOutlined />}
              message="Upgrade path is not compatible"
              description={errors.join('; ') || 'Compatibility check failed'}
            />
          )}
          {warnings.length > 0 && (
            <Alert type="warning" showIcon message="Warnings" description={warnings.join('; ')} />
          )}
          {compatResult.recommendations.length > 0 && (
            <Card type="inner" title="Recommendations" size="small">
              <ul>{compatResult.recommendations.map((item, index) => <li key={index}>{item}</li>)}</ul>
            </Card>
          )}
          <Space>
            <Button onClick={() => setStep(0)}>Back</Button>
            <Button type="primary" disabled={!compatResult.is_compatible} onClick={() => setStep(2)}>
              Choose strategy
            </Button>
          </Space>
        </Space>
      )}

      {step === 2 && (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Select
            style={{ width: '100%' }}
            value={strategy}
            onChange={setStrategy}
            options={[
              { label: 'In-place upgrade', value: 'inplace' },
              { label: 'Rolling upgrade', value: 'rolling' },
              { label: 'Logical migration', value: 'logical' },
            ]}
          />
          <Alert type="info" showIcon message={`Plan ID: ${planID || '-'}`} />
          <Space>
            <Button onClick={() => setStep(1)}>Back</Button>
            <Button type="primary" onClick={handleExecute} loading={upgrading}>
              Submit upgrade
            </Button>
          </Space>
        </Space>
      )}

      {step === 3 && (
        <Alert type="success" showIcon message="Upgrade task submitted. Check the task panel for progress." />
      )}
    </Card>
  )
}

export default UpgradeWizard
