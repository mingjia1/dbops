import React from 'react'
import { Alert, Button, Card, Col, Progress, Row, Space, Steps, Tag, Typography } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, ClusterOutlined, DeleteOutlined, ReloadOutlined } from '@ant-design/icons'
import { formatClusterRole } from '../services/roleDisplay'
import type { DeployResult } from '../services/deployHelpers'
import {
  isCompletedDeployStatus, isFailedDeployStatus, isPartialDeployStatus, isDestroyedDeployStatus, isTerminalDeployStatus,
  STAGE_ORDER, deploymentProgress, deploymentProgressStatus, deploymentStepStatus, stepStatusToAntd, stepProgressPercent,
  DEPLOY_SUBSTEPS, STEP_TYPE_CN,
} from '../services/deployHelpers'
import type { DeployStepView } from '../services/deployHelpers'

const { Text } = Typography

interface ClusterDeployProgressProps {
  activeDeployment: DeployResult
  currentStep: number
  onReturn: () => void
}

const renderVerticalStepProgress = (steps: DeployStepView[], overallProgress?: number) => (
  <Steps
    direction="vertical"
    size="small"
    current={steps.findIndex((step) => stepStatusToAntd(step.status) === 'process')}
    items={steps.map((step, idx) => {
      const status = stepStatusToAntd(step.status)
      const percent = stepProgressPercent(step, idx, steps, overallProgress)
      return {
        title: (
          <Space size={4} wrap>
            <span>{step.name || step.id || `步骤 ${idx + 1}`}</span>
            {step.type && <Tag color="default" style={{ fontSize: 10 }}>{STEP_TYPE_CN[step.type] || step.type}</Tag>}
            {step.target_node && <span style={{ color: '#888', fontSize: 12 }}>({step.target_node})</span>}
          </Space>
        ),
        description: (
          <Space direction="vertical" size={4} style={{ width: '100%' }}>
            <Progress
              percent={percent}
              size="small"
              status={status === 'error' ? 'exception' : status === 'finish' ? 'success' : 'active'}
            />
            <Space size={8} wrap>
              {step.message && <span style={{ fontSize: 12, color: '#666' }}>{step.message}</span>}
              {step.depends_on && step.depends_on.length > 0 && (
                <span style={{ fontSize: 12, color: '#888' }}>依赖: {step.depends_on.join(', ')}</span>
              )}
              {step.started_at && <span style={{ fontSize: 11, color: '#aaa' }}>开始 {new Date(step.started_at).toLocaleTimeString()}</span>}
              {step.completed_at && <span style={{ fontSize: 11, color: '#aaa' }}>完成 {new Date(step.completed_at).toLocaleTimeString()}</span>}
            </Space>
          </Space>
        ),
        status,
      }
    })}
  />
)

const ClusterDeployProgress: React.FC<ClusterDeployProgressProps> = ({ activeDeployment, currentStep, onReturn }) => {
  const dep = activeDeployment
  const stepIdx = dep.stage ? STAGE_ORDER.indexOf(dep.stage as typeof STAGE_ORDER[number]) : -1
  const displayStep = stepIdx >= 0 ? stepIdx : currentStep

  return (
    <Card
      title={
        <Space>
          <ClusterOutlined />
          <span>部署进度 - {dep.cluster_type?.toUpperCase()}</span>
          {!isTerminalDeployStatus(dep.status) && (
            <span style={{ fontSize: 12, color: '#1677ff' }}>
              <ReloadOutlined spin style={{ marginRight: 4 }} />
              实时更新中 (2s)
            </span>
          )}
        </Space>
      }
      style={{ marginTop: 16 }}
      extra={
        <Space>
          <Tag
            color={
              isCompletedDeployStatus(dep.status) ? 'success'
              : isPartialDeployStatus(dep.status) ? 'warning'
              : isFailedDeployStatus(dep.status) ? 'error'
              : isDestroyedDeployStatus(dep.status) ? 'default'
              : 'processing'
            }
            icon={
              isCompletedDeployStatus(dep.status) ? <CheckCircleOutlined />
              : isFailedDeployStatus(dep.status) ? <CloseCircleOutlined />
              : isDestroyedDeployStatus(dep.status) ? <DeleteOutlined />
              : !isTerminalDeployStatus(dep.status) ? <ReloadOutlined spin />
              : undefined
            }
          >
            {isCompletedDeployStatus(dep.status) ? '已完成'
              : isPartialDeployStatus(dep.status) ? '部分完成'
              : isFailedDeployStatus(dep.status) ? '失败'
              : isDestroyedDeployStatus(dep.status) ? '已销毁'
              : '运行中'}
          </Tag>
          {dep.finished_at && <span>完成于 {new Date(dep.finished_at).toLocaleString()}</span>}
        </Space>
      }
    >
      {/* Architecture info banner */}
      <div style={{ marginBottom: 16, display: 'flex', alignItems: 'center', gap: 12 }}>
        <Tag color={dep.cluster_type === 'ha' ? 'cyan' : dep.cluster_type === 'mha' ? 'blue' : dep.cluster_type === 'mgr' ? 'green' : 'orange'} style={{ fontSize: 14, padding: '2px 12px' }}>
          <ClusterOutlined style={{ marginRight: 4 }} />
          {dep.cluster_type?.toUpperCase()}
        </Tag>
        {dep.nodes && dep.nodes.length > 0 && (
          <Space size={4}>
            {dep.nodes.map((node, idx) => (
              <Tag key={idx} color={node.role === 'master' || node.role === 'primary' || node.role === 'bootstrap' ? 'blue' : node.role === 'manager' ? 'purple' : 'default'}>
                {formatClusterRole(dep.cluster_type, node.role)}
              </Tag>
            ))}
          </Space>
        )}
      </div>

      {/* 5-stage progress bar */}
      <Steps
        current={displayStep}
        size="small"
        items={STAGE_ORDER.map((title, idx) => ({
          title,
          description: idx === displayStep && !isTerminalDeployStatus(dep.status)
            ? <span style={{ fontSize: 11, color: '#1677ff' }}>进行中...</span>
            : idx < displayStep
              ? <span style={{ fontSize: 11, color: '#52c41a' }}>已完成</span>
              : undefined,
        }))}
        status={deploymentStepStatus(dep.status)}
      />

      {/* Overall progress bar */}
      <div style={{ marginTop: 16, display: 'flex', alignItems: 'center', gap: 12 }}>
        <div style={{ flex: 1 }}>
          <Progress
            percent={deploymentProgress(dep.status, dep.progress)}
            status={deploymentProgressStatus(dep.status)}
            strokeColor={{
              '0%': '#108ee9',
              '100%': '#87d068',
            }}
          />
        </div>
        <span style={{ fontSize: 24, fontWeight: 600, color: '#333', minWidth: 48, textAlign: 'right' }}>
          {deploymentProgress(dep.status, dep.progress)}%
        </span>
      </div>

      {/* Status message */}
      <Alert
        type={
          isFailedDeployStatus(dep.status) ? 'error'
          : isCompletedDeployStatus(dep.status) ? 'success'
          : isPartialDeployStatus(dep.status) ? 'warning'
          : 'info'
        }
        message={
          <Space>
            <span>{dep.message || '等待后端返回状态...'}</span>
            {!isTerminalDeployStatus(dep.status) && (
              <span style={{ fontSize: 11, color: '#888' }}>
                ({STAGE_ORDER[displayStep] || dep.stage || '初始化中'})
              </span>
            )}
          </Space>
        }
        showIcon
        style={{ marginBottom: 16 }}
      />

      {/* Detailed steps timeline */}
      {dep.steps && dep.steps.length > 0 && (
        <div style={{ marginTop: 16 }}>
          <strong style={{ display: 'block', marginBottom: 8 }}>详细步骤</strong>
          {renderVerticalStepProgress(dep.steps, dep.progress)}
        </div>
      )}

      {/* Fallback substeps when backend doesn't return steps yet */}
      {(!dep.steps || dep.steps.length === 0) && (
        <div style={{ marginTop: 16 }}>
          <strong style={{ display: 'block', marginBottom: 8 }}>当前阶段子步骤</strong>
          <div style={{ marginTop: 8 }}>
            {(DEPLOY_SUBSTEPS[dep.stage || ''] || DEPLOY_SUBSTEPS['环境检查']).map((substep, idx) => {
              const substeps = DEPLOY_SUBSTEPS[dep.stage || ''] || DEPLOY_SUBSTEPS['环境检查']
              const isLast = idx === substeps.length - 1
              return (
                <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4, opacity: isLast && !isCompletedDeployStatus(dep.status) ? 1 : idx < substeps.length - 1 ? 0.6 : 0.4 }}>
                  {idx < substeps.length - 1 ? (
                    <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 14 }} />
                  ) : !isTerminalDeployStatus(dep.status) ? (
                    <ReloadOutlined spin style={{ color: '#1677ff', fontSize: 14 }} />
                  ) : (
                    <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 14 }} />
                  )}
                  <span style={{ color: isLast && !isCompletedDeployStatus(dep.status) ? '#333' : '#999' }}>{substep}</span>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Node progress cards */}
      {dep.nodes && dep.nodes.length > 0 && (
        <div style={{ marginTop: 16 }}>
          <strong style={{ display: 'block', marginBottom: 8 }}>节点进度 ({dep.nodes.length})</strong>
          <Row gutter={[12, 12]}>
            {dep.nodes.map((node, idx) => (
              <Col span={8} key={idx}>
                <Card
                  size="small"
                  style={{
                    borderLeft: `3px solid ${
                      node.role === 'master' || node.role === 'primary' || node.role === 'bootstrap' ? '#1677ff'
                      : node.role === 'manager' ? '#722ed1'
                      : '#52c41a'
                    }`,
                  }}
                  title={
                    <Space size={4}>
                      <span style={{ fontSize: 13, fontWeight: 500 }}>{node.name || node.instance_id || `节点 ${idx + 1}`}</span>
                      <span style={{ fontSize: 11, color: '#888' }}>{node.host || '-'}:{node.port || '-'}</span>
                    </Space>
                  }
                  extra={
                    <Tag color={node.role === 'master' || node.role === 'primary' || node.role === 'bootstrap' ? 'blue' : node.role === 'manager' ? 'purple' : 'default'}>
                      {formatClusterRole(dep.cluster_type, node.role)}
                    </Tag>
                  }
                >
                  <Space direction="vertical" size={4} style={{ width: '100%' }}>
                    <Space>
                      <span style={{ fontSize: 12, color: '#666' }}>状态: </span>
                      <Tag
                        color={
                          node.status === 'completed' || node.status === 'healthy' ? 'success'
                          : node.status === 'running' || node.status === 'deploying' ? 'processing'
                          : node.status === 'failed' ? 'error'
                          : 'default'
                        }
                        style={{ fontSize: 11 }}
                      >
                        {node.status || 'pending'}
                      </Tag>
                      {node.current_step && (
                        <span style={{ fontSize: 11, color: '#888' }}>{node.current_step}</span>
                      )}
                    </Space>
                    {typeof node.progress === 'number' && (
                      <Progress percent={node.progress} size="small" />
                    )}
                    {node.message && (
                      <div style={{ color: '#888', fontSize: 11, lineHeight: 1.4 }}>{node.message}</div>
                    )}
                  </Space>
                </Card>
              </Col>
            ))}
          </Row>
        </div>
      )}

      {/* Live log viewer */}
      {dep.logs && dep.logs.length > 0 && (
        <div style={{ marginTop: 16 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
            <strong>部署日志 ({dep.logs.length})</strong>
            {!isTerminalDeployStatus(dep.status) && (
              <span style={{ fontSize: 11, color: '#1677ff' }}>
                <ReloadOutlined spin style={{ marginRight: 4 }} />
                实时
              </span>
            )}
          </div>
          <div
            style={{
              background: '#1e1e1e',
              color: '#d4d4d4',
              padding: 12,
              borderRadius: 6,
              maxHeight: 200,
              overflow: 'auto',
              fontFamily: '"Cascadia Code", "Fira Code", "Consolas", monospace',
              fontSize: 12,
              lineHeight: 1.6,
            }}
          >
            {dep.logs.map((log, idx) => (
              <div key={idx} style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                <span style={{ color: '#888', marginRight: 8 }}>[{idx + 1}]</span>
                {log.includes('ERROR') || log.includes('failed') || log.includes('错误') ? (
                  <span style={{ color: '#f56c6c' }}>{log}</span>
                ) : log.includes('completed') || log.includes('成功') ? (
                  <span style={{ color: '#67c23a' }}>{log}</span>
                ) : (
                  <span>{log}</span>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Restart/retry button for terminal states */}
      {isTerminalDeployStatus(dep.status) && (
        <div style={{ marginTop: 16, textAlign: 'center' }}>
          <Button
            icon={<ReloadOutlined />}
            onClick={onReturn}
          >
            返回部署表单
          </Button>
        </div>
      )}
    </Card>
  )
}

export default ClusterDeployProgress
