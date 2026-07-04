import React from 'react'
import { Button, Descriptions, Empty, Modal, Space, Table, Tag } from 'antd'
import { EyeOutlined } from '@ant-design/icons'
import { formatClusterRole } from '../services/roleDisplay'
import type { ArchType, DeployStepView } from '../services/deployHelpers'
import { STEP_TYPE_CN } from '../services/deployHelpers'

interface PlanPreviewModalProps {
  open: boolean
  onClose: () => void
  data: any
  arch: ArchType
}

const PlanPreviewModal: React.FC<PlanPreviewModalProps> = ({ open, onClose, data, arch }) => {
  const renderPreviewSteps = (steps: DeployStepView[]) => (
    <div>
      {steps.map((step, idx) => (
        <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '4px 0', borderBottom: '1px solid #f0f0f0' }}>
          <span style={{ fontSize: 13, fontWeight: 500 }}>{step.name || step.id || `步骤 ${idx + 1}`}</span>
          {step.type && <Tag color="default" style={{ fontSize: 10, lineHeight: '16px' }}>{STEP_TYPE_CN[step.type] || step.type}</Tag>}
          {step.target_node && <span style={{ color: '#888', fontSize: 12 }}>({step.target_node})</span>}
          {step.message && <span style={{ fontSize: 12, color: '#666', marginLeft: 8 }}>{step.message}</span>}
        </div>
      ))}
    </div>
  )

  return (
    <Modal
      title={
        <Space>
          <EyeOutlined />
          <span>部署计划预览 - {arch?.toUpperCase()}</span>
        </Space>
      }
      open={open}
      onCancel={onClose}
      width={720}
      footer={<Button onClick={onClose}>关闭</Button>}
      destroyOnClose
      styles={{ body: { padding: '16px 20px' } }}
    >
      {data ? (
        <div>
          <Descriptions size="small" column={3} bordered style={{ marginBottom: 12 }}>
            <Descriptions.Item label="部署ID" span={2}>{data.deployment_id || data.id || '-'}</Descriptions.Item>
            <Descriptions.Item label="架构">
              <Tag color={data.cluster_type === 'ha' ? 'cyan' : data.cluster_type === 'mha' ? 'blue' : data.cluster_type === 'mgr' ? 'green' : 'orange'} style={{ margin: 0 }}>
                {(data.cluster_type || '').toUpperCase()}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="模式">
              <Tag style={{ margin: 0 }}>{data.mode || 'real'}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="节点">{data.nodes?.length || 0}</Descriptions.Item>
            <Descriptions.Item label="步骤">{data.steps?.length || 0}</Descriptions.Item>
            {data.parameters?.mysql_version && (
              <Descriptions.Item label="MySQL 版本">{data.parameters.mysql_version}</Descriptions.Item>
            )}
          </Descriptions>

          <div style={{ marginBottom: 12 }}>
            <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 6 }}>节点列表</div>
            <Table
              size="small"
              pagination={false}
              showHeader={false}
              columns={[
                { title: 'Host', dataIndex: 'host', key: 'host', width: 130 },
                { title: '角色', dataIndex: 'role', key: 'role', width: 80, render: (role: string) => <Tag style={{ margin: 0 }}>{formatClusterRole(arch, role)}</Tag> },
                { title: '端口', key: 'ports', width: 110, render: (_: any, record: any) => `${record.mysql_port || '-'}${record.agent_port ? ` / ${record.agent_port}` : ''}` },
                { title: '数据目录', dataIndex: 'data_dir', key: 'data_dir', render: (v: string) => v || '-', ellipsis: true },
                { title: 'Server ID', dataIndex: 'server_id', key: 'server_id', width: 72, render: (v: number) => v || '-' },
              ]}
              dataSource={data.nodes || []}
              rowKey={(row: any, index?: number) => row.id || row.host || `node-${index}`}
            />
          </div>

          <div style={{ marginBottom: 12 }}>
            <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 6 }}>执行步骤</div>
            {(data.steps && data.steps.length > 0)
              ? renderPreviewSteps(data.steps.map((step: any) => ({ ...step, status: step.status || 'planned' })))
              : <span style={{ color: '#999', fontSize: 12 }}>暂无步骤信息</span>
            }
          </div>

          {data.parameters && Object.keys(data.parameters).length > 0 && (
            <div>
              <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 6 }}>部署参数</div>
              <Descriptions size="small" column={2} bordered>
                {Object.entries(data.parameters).map(([key, value]: [string, any]) => (
                  <Descriptions.Item label={key} key={key}>
                    {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                  </Descriptions.Item>
                ))}
              </Descriptions>
            </div>
          )}
        </div>
      ) : (
        <Empty description="无法加载部署计划" />
      )}
    </Modal>
  )
}

export default PlanPreviewModal
