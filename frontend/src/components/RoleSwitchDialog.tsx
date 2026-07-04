import React, { useState, useEffect } from 'react'
import { Modal, Select, Button, Space, Alert, Spin, Descriptions, message } from 'antd'
import { SwapOutlined } from '@ant-design/icons'
import { roleSwitchApi } from '../services/api'

interface NodeInfo {
  instance_id: string
  name: string
  role: string
  host?: string
  status?: string
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
  const [step, setStep] = useState<'select' | 'confirm' | 'executing' | 'done'>('select')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (open) {
      setStep('select')
      setTargetNode('')
    }
  }, [open])

  const replicaNodes = nodes.filter(n => ['replica', 'secondary', 'slave'].includes((n.role || '').toLowerCase()))
  const targetRoleForNode = (node?: NodeInfo) => {
    const role = (node?.role || '').toLowerCase()
    return role === 'slave' || role === 'replica' ? 'master' : 'primary'
  }

  const handleExecute = async () => {
    if (!targetNode) { message.warning('请选择目标主节点'); return }
    setStep('executing')
    setLoading(true)
    try {
      const selectedNode = nodes.find(n => n.instance_id === targetNode)
      await roleSwitchApi.switch({
        cluster_id: clusterID,
        instance_id: targetNode,
        target_role: targetRoleForNode(selectedNode),
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
          <Button type="primary" block onClick={() => {
            if (!targetNode) { message.warning('请选择目标主节点'); return }
            setStep('confirm')
          }}>
            下一步
          </Button>
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
            <Button onClick={() => setStep('select')}>返回</Button>
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
