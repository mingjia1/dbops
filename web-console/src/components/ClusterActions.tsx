import React, { useState } from 'react'
import { Card, Button, Space, Modal, Select, InputNumber, message, Popconfirm } from 'antd'
import {
  PlusOutlined, MinusOutlined, ReloadOutlined, DeleteOutlined,
} from '@ant-design/icons'
import api from '../services/api'

interface ClusterActionsProps {
  clusterID: string
  clusterName?: string
  archType?: string
  flavor?: string
  onActionComplete?: (action: string, result: any) => void
}

export const ClusterActions: React.FC<ClusterActionsProps> = ({
  clusterID, clusterName, archType, flavor, onActionComplete,
}) => {
  const [scaleOutOpen, setScaleOutOpen] = useState(false)
  const [scaleInOpen, setScaleInOpen] = useState(false)
  const [loading, setLoading] = useState<string | null>(null)
  const [newNodeCount, setNewNodeCount] = useState(1)

  const handleAction = async (action: string, payload?: any) => {
    setLoading(action)
    try {
      const res = await api.post(`/clusters/${clusterID}/${action}`, payload)
      message.success(`${action} 成功`)
      onActionComplete?.(action, res.data)
    } catch (err: any) {
      message.error(`${action} 失败: ${err.message}`)
    } finally {
      setLoading(null)
    }
  }

  return (
    <Card title={`集群操作: ${clusterName || clusterID}`} size="small">
      <Space wrap>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => setScaleOutOpen(true)}
          loading={loading === 'scale-out'}
        >
          扩容
        </Button>
        <Button
          icon={<MinusOutlined />}
          onClick={() => setScaleInOpen(true)}
          loading={loading === 'scale-in'}
        >
          缩容
        </Button>
        <Button
          icon={<ReloadOutlined />}
          onClick={() => handleAction('rebuild')}
          loading={loading === 'rebuild'}
        >
          重建
        </Button>
        <Popconfirm
          title="确认销毁集群？此操作不可恢复。"
          onConfirm={() => handleAction('destroy')}
          okText="确认销毁"
          cancelText="取消"
          okButtonProps={{ danger: true }}
        >
          <Button danger icon={<DeleteOutlined />} loading={loading === 'destroy'}>
            销毁
          </Button>
        </Popconfirm>
      </Space>

      <Modal
        title="扩容 — 添加节点"
        open={scaleOutOpen}
        onCancel={() => setScaleOutOpen(false)}
        onOk={() => {
          handleAction('scale-out', { node_count: newNodeCount })
          setScaleOutOpen(false)
        }}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <span>添加副本节点数量：</span>
          <InputNumber min={1} max={10} value={newNodeCount} onChange={v => setNewNodeCount(v || 1)} />
        </Space>
      </Modal>

      <Modal
        title="缩容 — 移除节点"
        open={scaleInOpen}
        onCancel={() => setScaleInOpen(false)}
        onOk={() => {
          handleAction('scale-in', { remove_count: 1 })
          setScaleInOpen(false)
        }}
      >
        <p>将从集群中移除一个副本节点（主节点不可移除，需先执行角色切换）。</p>
      </Modal>
    </Card>
  )
}

export default ClusterActions
