import React, { useState } from 'react'
import { Card, Button, Space, Modal, Select, message, Popconfirm } from 'antd'
import {
  PlusOutlined, MinusOutlined, ReloadOutlined, DeleteOutlined,
} from '@ant-design/icons'
import api, { hostApi, type Host } from '../services/api'

interface NodeInfo {
  instance_id: string
  name: string
  role: string
  host?: string
}

interface ClusterActionsProps {
  clusterID: string
  clusterName?: string
  archType?: string
  flavor?: string
  nodes?: NodeInfo[]
  onActionComplete?: (action: string, result: any) => void
}

export const ClusterActions: React.FC<ClusterActionsProps> = ({
  clusterID, clusterName, nodes = [], onActionComplete,
}) => {
  const [scaleOutOpen, setScaleOutOpen] = useState(false)
  const [scaleInOpen, setScaleInOpen] = useState(false)
  const [rebuildOpen, setRebuildOpen] = useState(false)
  const [loading, setLoading] = useState<string | null>(null)
  const [hosts, setHosts] = useState<Host[]>([])
  const [scaleOutHostIDs, setScaleOutHostIDs] = useState<string[]>([])
  const [removeNodeId, setRemoveNodeId] = useState<string>('')
  const [rebuildNodeId, setRebuildNodeId] = useState<string>('')

  const handleAction = async (action: string, payload?: any) => {
    setLoading(action)
    try {
      const res = await api.post(`/deployments/${clusterID}/${action}`, payload)
      message.success(`${action} 成功`)
      onActionComplete?.(action, res.data)
    } catch (err: any) {
      message.error(`${action} 失败: ${err?.response?.data?.message || err.message}`)
    } finally {
      setLoading(null)
    }
  }

  const loadHostsForScaleOut = async () => {
    if (hosts.length > 0) return
    try {
      const res: any = await hostApi.list(1000, 0)
      setHosts(res?.data || [])
    } catch (err: any) {
      message.error(`加载主机失败: ${err?.response?.data?.message || err.message}`)
    }
  }

  const existingNodeHosts = new Set(nodes.map((n) => n.host).filter(Boolean))
  const availableHostOptions = hosts
    .filter((host) => !existingNodeHosts.has(host.address))
    .map((host) => ({
      label: `${host.name} (${host.address})`,
      value: host.id,
    }))

  return (
    <Card title={`集群操作: ${clusterName || clusterID}`} size="small">
      <Space wrap>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => {
            loadHostsForScaleOut()
            setScaleOutOpen(true)
          }}
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
          onClick={() => setRebuildOpen(true)}
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
        title="扩容 - 添加节点"
        open={scaleOutOpen}
        onCancel={() => {
          setScaleOutOpen(false)
          setScaleOutHostIDs([])
        }}
        onOk={() => {
          if (scaleOutHostIDs.length === 0) {
            message.warning('请选择要加入集群的主机')
            return
          }
          handleAction('scale-out', { node_count: scaleOutHostIDs.length, host_ids: scaleOutHostIDs })
          setScaleOutOpen(false)
          setScaleOutHostIDs([])
        }}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <span>选择新增节点主机：</span>
          <Select
            mode="multiple"
            style={{ width: '100%' }}
            placeholder="选择要扩容的主机"
            value={scaleOutHostIDs}
            onChange={setScaleOutHostIDs}
            options={availableHostOptions}
          />
        </Space>
      </Modal>

      <Modal
        title="缩容 - 移除节点"
        open={scaleInOpen}
        onCancel={() => setScaleInOpen(false)}
        onOk={() => {
          if (!removeNodeId) {
            message.warning('请选择要移除的节点')
            return
          }
          handleAction('scale-in', { remove_node_id: removeNodeId })
          setScaleInOpen(false)
          setRemoveNodeId('')
        }}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <span>选择要移除的副本节点（主节点不可移除，需要先执行角色切换）：</span>
          <Select
            style={{ width: '100%' }}
            placeholder="选择节点"
            value={removeNodeId || undefined}
            onChange={setRemoveNodeId}
            options={nodes
              .filter((n) => n.role === 'replica' || n.role === 'secondary')
              .map((n) => ({
                label: `${n.name || n.instance_id} (${n.host || ''}) - ${n.role}`,
                value: n.instance_id,
              }))}
          />
        </Space>
      </Modal>

      <Modal
        title="重建 - 重建节点"
        open={rebuildOpen}
        onCancel={() => setRebuildOpen(false)}
        onOk={() => {
          if (!rebuildNodeId) {
            message.warning('请选择要重建的节点')
            return
          }
          handleAction('rebuild', { node_id: rebuildNodeId })
          setRebuildOpen(false)
          setRebuildNodeId('')
        }}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <span>选择要重建的节点：</span>
          <Select
            style={{ width: '100%' }}
            placeholder="选择节点"
            value={rebuildNodeId || undefined}
            onChange={setRebuildNodeId}
            options={nodes.map((n) => ({
              label: `${n.name || n.instance_id} (${n.host || ''}) - ${n.role}`,
              value: n.instance_id,
            }))}
          />
        </Space>
      </Modal>
    </Card>
  )
}

export default ClusterActions
