import React from 'react'
import { Card, Descriptions, Tag, Divider, Progress, Steps } from 'antd'
import {
  MigrationTask, MigrationProgressStep,
  migrationStatusColor, migrationProgressStatus,
  getCurrentSubstages,
} from '../services/migrationHelpers'

interface MigrationProgressMonitorProps {
  task: MigrationTask
  progressDetails: MigrationProgressStep[]
}

const stepStatusToAntd = (status?: string): 'finish' | 'process' | 'error' | 'wait' => {
  if (status === 'completed') return 'finish'
  if (status === 'running') return 'process'
  if (status === 'failed') return 'error'
  return 'wait'
}

const MigrationProgressMonitor: React.FC<MigrationProgressMonitorProps> = ({ task, progressDetails }) => {
  if (!task) return null

  const currentSubstages = getCurrentSubstages(task.status)

  return (
    <Card title="迁移进度监控" style={{ marginTop: 16 }}>
      <Descriptions column={2} bordered>
        <Descriptions.Item label="任务ID">{task.id}</Descriptions.Item>
        <Descriptions.Item label="迁移类型">
          <Tag color="blue">{task.migration_type || task.strategy}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label="源实例">{task.source_instance || task.source_instance_id}</Descriptions.Item>
        <Descriptions.Item label="目标实例">{task.target_instance || task.target_instance_id}</Descriptions.Item>
        <Descriptions.Item label="状态">
          <Tag color={migrationStatusColor(task.status)}>
            {task.status}
          </Tag>
        </Descriptions.Item>
        <Descriptions.Item label="开始时间">{task.started_at}</Descriptions.Item>
        {(task.error || task.error_message) && (
          <Descriptions.Item label="错误信息" span={2}>{task.error || task.error_message}</Descriptions.Item>
        )}
      </Descriptions>

      <Divider />

      <div style={{ marginBottom: 8 }}>
        <strong>总体进度</strong>
      </div>
      <Progress percent={task.progress} status={migrationProgressStatus(task.status)} />

      <Divider />

      {task.steps && task.steps.length > 0 ? (
        <div>
          <strong>详细步骤</strong>
          <Steps direction="vertical" size="small" style={{ marginTop: 8 }} current={task.steps.findIndex((s) => s.status === 'running')}>
            {task.steps.map((step, idx) => (
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
                status={stepStatusToAntd(step.status)}
              />
            ))}
          </Steps>
        </div>
      ) : (
        <div>
          <strong>迁移阶段</strong>
          <Steps current={-1} direction="vertical" style={{ marginTop: 8 }}>
            {progressDetails.map((item, index) => (
              <Steps.Step
                key={index}
                title={item.stage}
                description={
                  <div>
                    <Progress percent={item.progress} size="small" />
                    <span>{item.details}</span>
                  </div>
                }
              />
            ))}
          </Steps>
        </div>
      )}

      <Divider />
      <strong>当前阶段子步骤</strong>
      <div style={{ marginTop: 8 }}>
        {currentSubstages.map((substep, idx) => (
          <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
            {idx < currentSubstages.length - 1 ? (
              <span style={{ color: '#52c41a' }}>&#10003;</span>
            ) : (
              <span style={{ color: '#1677ff' }}>&#9679;</span>
            )}
            <span>{substep}</span>
          </div>
        ))}
      </div>

      {task.logs && task.logs.length > 0 && (
        <>
          <Divider />
          <strong>迁移日志</strong>
          <div style={{ background: '#1e1e1e', color: '#d4d4d4', padding: 12, borderRadius: 6, maxHeight: 200, overflow: 'auto', fontFamily: 'monospace', fontSize: 12, marginTop: 8 }}>
            {task.logs.map((log, idx) => (
              <div key={idx}>{log}</div>
            ))}
          </div>
        </>
      )}
    </Card>
  )
}

export default MigrationProgressMonitor
