import React, { useState, useEffect } from 'react'
import { Modal, Select, Button, Space, Alert, Spin, Descriptions, Tag, message } from 'antd'
import { SwapOutlined, CheckCircleOutlined, WarningOutlined } from '@ant-design/icons'
import api from '../services/api'

interface NodeInfo {
  instance_id: string
  name: string
  role: string
  host?: string
  status?: string
}

interface PreflightResult {
  cluster_id: string
  target_master_id: string
  warnings: string[]
  errors: string[]
  is_ready: boolean
}

interface RoleSwitchDialogProps {
  open: boolean
  clusterID: string
  nodes: NodeInfo[]
  onClose: () => void
  onComplete?: () => void
}

export const RoleSwitchDialog: React.FC<RoleSwitchDialogProps> = ({
  open, clusterID, nodes, onClose, onComplete,
}) => {
  const [targetNode, setTargetNode] = useState<string>('')
  const [step, setStep] = useState<'select' | 'preflight' | 'confirm' | 'executing' | 'done'>('select')
  const [preflight, setPreflight] = useState<PreflightResult | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (open) {
      setStep('select')
      setTargetNode('')
      setPreflight(null)
    }
  }, [open])

  const replicaNodes = nodes.filter(n => n.role === 'replica' || n.role === 'secondary')

  const handlePreflight = async () => {
    if (!targetNode) { message.warning('请选择目标主节点'); return }
    setStep('preflight')
    setLoading(true)
    try {
      const res = await api.post(`/clusters/${clusterID}/switch/preflight`, {
        target_master_id: targetNode,
      })
      setPreflight(res.data?.data || null)
    } catch (err: any) {
      message.error(`预检失败: ${err.message}`)
      setStep('select')
    } finally {
      setLoading(false)
    }
  }

  const handleExecute = async () => {
    setStep('executing')
    setLoading(true)
    try {
      await api.post(`/clusters/${clusterID}/switch`, {
        new_master_id: targetNode,
      })
      message.success('角色切换完成')
      setStep('done')
      onComplete?.()
    } catch (err: any) {
      message.error(`切换失败: ${err.message}`)
      setStep('confirm')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Modal
      title={<><SwapOutlined /> 角色切换</>}
      open={open}
      onCancel={onClose}
      width={560}
      footer={null}
    >
      {step === 'select' && (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <div>选择新的主节点（当前副本）：</div>
          <Select
            style={{ width: '100%' }}
            placeholder="选择目标主节点"
            value={targetNode || undefined}
            onChange={setTargetNode}
            options={replicaNodes.map(n => ({
              label: `${n.name || n.instance_id}  (${n.host || ''})  — ${n.status || ''}`,
              value: n.instance_id,
            }))}
          />
          <Button type="primary" block onClick={handlePreflight}>
            预检
          </Button>
        </Space>
      )}

      {step === 'preflight' && loading && <Spin tip="预检中..." />}

      {step === 'preflight' && preflight && !loading && (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          {preflight.is_ready ? (
            <Alert type="success" showIcon icon={<CheckCircleOutlined />}
              message="预检通过，可以安全切换" />
          ) : (
            <Alert type="error" showIcon icon={<WarningOutlined />}
              message="预检未通过" description={preflight.errors.join('; ')} />
          )}
          {preflight.warnings.length > 0 && (
            <Alert type="warning" showIcon message="警告"
              description={preflight.warnings.join('; ')} />
          )}
          <Space>
            <Button onClick={() => setStep('select')}>返回</Button>
            <Button type="primary" danger disabled={!preflight.is_ready}
              onClick={() => setStep('confirm')}>
              确认切换
            </Button>
          </Space>
        </Space>
      )}

      {step === 'confirm' && (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Alert type="warning" showIcon message="确认角色切换"
            description="此操作将把主节点切换到选定的副本。请确保已了解影响。" />
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label="集群 ID">{clusterID}</Descriptions.Item>
            <Descriptions.Item label="目标主节点">
              {nodes.find(n => n.instance_id === targetNode)?.name || targetNode}
            </Descriptions.Item>
          </Descriptions>
          <Space>
            <Button onClick={() => setStep('preflight')}>返回</Button>
            <Button type="primary" danger loading={loading} onClick={handleExecute}>
              执行切换
            </Button>
          </Space>
        </Space>
      )}

      {step === 'executing' && <Spin tip="正在执行角色切换..." />}

      {step === 'done' && (
        <Space direction="vertical" style={{ width: '100%' }}>
          <Alert type="success" showIcon message="角色切换已完成" />
          <Button type="primary" block onClick={onClose}>关闭</Button>
        </Space>
      )}
    </Modal>
  )
}

export default RoleSwitchDialog
