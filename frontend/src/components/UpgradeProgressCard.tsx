import React from 'react'
import { Card, Space, Tag, Steps, Progress, Divider } from 'antd'
import {
  ActiveUpgrade,
  isCompletedUpgradeStatus, isFailedUpgradeStatus,
  buildUpgradeSteps, upgradeStagesFor,
  getUpgradeSubsteps,
} from '../services/upgradeHelpers'

interface UpgradeProgressCardProps {
  activeUpgrade: ActiveUpgrade
  upgradeStep: number
  planResult: any | null
}

const UpgradeProgressCard: React.FC<UpgradeProgressCardProps> = ({ activeUpgrade, upgradeStep, planResult }) => {
  const activeUpgradeStages = upgradeStagesFor(activeUpgrade)
  const activeUpgradeSteps = buildUpgradeSteps(activeUpgrade)
  const activeUpgradeCurrentStage = activeUpgradeStages[upgradeStep] || activeUpgradeStages[0]
  const activeUpgradeSubsteps = React.useMemo(
    () => getUpgradeSubsteps(planResult, activeUpgradeCurrentStage),
    [planResult, activeUpgradeCurrentStage],
  )

  return (
    <Card
      title="升级进度"
      style={{ marginTop: 16 }}
      extra={
        <Space>
          <Tag color={isCompletedUpgradeStatus(activeUpgrade.status) ? 'success' : isFailedUpgradeStatus(activeUpgrade.status) ? 'error' : 'processing'}>
            {activeUpgrade.status}
          </Tag>
          {activeUpgrade.finished_at && <span>完成于 {new Date(activeUpgrade.finished_at).toLocaleString()}</span>}
        </Space>
      }
    >
      <Steps
        current={upgradeStep}
        size="small"
        items={activeUpgradeStages.map((title) => ({ title }))}
        status={isFailedUpgradeStatus(activeUpgrade.status) ? 'error' : isCompletedUpgradeStatus(activeUpgrade.status) ? 'finish' : 'process'}
      />
      <Progress
        percent={activeUpgrade.progress}
        status={isCompletedUpgradeStatus(activeUpgrade.status) ? 'success' : isFailedUpgradeStatus(activeUpgrade.status) ? 'exception' : 'active'}
        style={{ marginTop: 16 }}
      />
      <div style={{ marginTop: 8, color: '#666' }}>{activeUpgrade.message}</div>

      <div style={{ marginTop: 16 }}>
        <strong>详细步骤</strong>
        <Steps direction="vertical" size="small" style={{ marginTop: 8 }} current={activeUpgradeSteps.findIndex((s) => s.status === 'running')}>
          {activeUpgradeSteps.map((step, idx) => (
            <Steps.Step
              key={idx}
              title={step.name}
              description={
                <div>
                  <div style={{ color: '#888', fontSize: 12 }}>
                    {step.message || ''}
                    {step.started_at && ` (${new Date(step.started_at).toLocaleTimeString()})`}
                    {step.completed_at && ` -> ${new Date(step.completed_at).toLocaleTimeString()}`}
                  </div>
                </div>
              }
              status={step.status === 'completed' ? 'finish' : step.status === 'running' ? 'process' : step.status === 'failed' ? 'error' : 'wait'}
            />
          ))}
        </Steps>
      </div>

      <div style={{ marginTop: 16 }}>
        <strong>当前阶段子步骤</strong>
        <div style={{ marginTop: 8 }}>
          {activeUpgradeSubsteps.map((substep: string, idx: number) => (
            <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
              {idx < activeUpgradeSubsteps.length - 1 ? (
                <span style={{ color: '#52c41a' }}>&#10003;</span>
              ) : (
                <span style={{ color: '#1677ff' }}>&#9679;</span>
              )}
              <span>{substep}</span>
            </div>
          ))}
        </div>
      </div>

      {activeUpgrade.logs && activeUpgrade.logs.length > 0 && (
        <div style={{ marginTop: 16 }}>
          <strong>升级日志</strong>
          <div style={{ background: '#1e1e1e', color: '#d4d4d4', padding: 12, borderRadius: 6, maxHeight: 200, overflow: 'auto', fontFamily: 'monospace', fontSize: 12, marginTop: 8 }}>
            {activeUpgrade.logs.map((log, idx) => (
              <div key={idx}>{log}</div>
            ))}
          </div>
        </div>
      )}
    </Card>
  )
}

export default UpgradeProgressCard
