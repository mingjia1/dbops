import React, { useEffect, useMemo, useState } from 'react'
import { Alert, Button, Card, Descriptions, Form, Result, Select, Space, Table, Tag, message } from 'antd'
import { HistoryOutlined, RetweetOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { instanceApi, roleSwitchApi, type Instance } from '../services/api'

interface SwitchResult {
  id?: string
  cluster_id: string
  cluster_type?: string
  instance_id: string
  instance_host?: string
  old_role: string
  new_role: string
  status: string
  message: string
  old_master_id: string
  new_master_id: string
  occurred_at?: string
  started_at?: string
  completed_at?: string
}

const isFailedSwitchStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(normalized)
}

const isCompletedSwitchStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['completed', 'success', 'succeeded', 'ok'].includes(normalized)
}

const isSkippedSwitchStatus = (status?: string) => (status || '').toLowerCase() === 'skipped'

const switchStatusColor = (status?: string) => {
  if (isCompletedSwitchStatus(status)) return 'success'
  if (isFailedSwitchStatus(status)) return 'error'
  if (isSkippedSwitchStatus(status)) return 'default'
  return 'processing'
}

const TARGET_ROLES: Record<string, string[]> = {
  ha: ['master', 'slave'],
  mha: ['master', 'slave'],
  mgr: ['primary', 'secondary'],
  pxc: ['primary', 'secondary'],
}

const normalizeArch = (value?: string) => {
  const v = (value || '').toLowerCase()
  if (['ha', 'mha', 'mgr', 'pxc'].includes(v)) return v
  if (v === 'master-slave' || v === 'replication') return 'ha'
  return ''
}

const instanceArch = (instance: Instance) =>
  normalizeArch(instance.status?.replication_status || instance.topology?.replication_mode)

const RoleSwitch: React.FC = () => {
  const [instances, setInstances] = useState<Instance[]>([])
  const [clusterId, setClusterId] = useState<string | undefined>(undefined)
  const [targetRole, setTargetRole] = useState<string | undefined>(undefined)
  const [selectedInstance, setSelectedInstance] = useState<string | undefined>(undefined)
  const [submitting, setSubmitting] = useState(false)
  const [history, setHistory] = useState<SwitchResult[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)
  const [form] = Form.useForm()

  useEffect(() => {
    instanceApi.list(1000, 0).then((res: any) => setInstances(res?.data || [])).catch(() => {})
  }, [])

  const clusters = useMemo(() => {
    const grouped = new Map<string, Instance[]>()
    instances.forEach((inst) => {
      if (!inst.cluster_id) return
      grouped.set(inst.cluster_id, [...(grouped.get(inst.cluster_id) || []), inst])
    })
    return Array.from(grouped.entries()).map(([id, items]) => {
      const arch = instanceArch(items.find((item) => instanceArch(item)) || items[0])
      return { id, arch, items }
    }).filter((item) => item.arch)
  }, [instances])

  const selectedCluster = clusters.find((item) => item.id === clusterId)
  const selectedArch = selectedCluster?.arch || ''
  const clusterInstances = useMemo(
    () => instances.filter((i) => i.cluster_id === clusterId && instanceArch(i) === selectedArch),
    [instances, clusterId, selectedArch],
  )
  const selected = clusterInstances.find((item) => item.id === selectedInstance)
  const selectedRole = selected?.status?.role || ''
  const targetRoleOptions = (TARGET_ROLES[selectedArch] || [])
    .filter((role) => role !== selectedRole)
    .map((role) => ({ value: role, label: role }))

  const fetchHistory = async (cid: string) => {
    setHistoryLoading(true)
    try {
      const res: any = await roleSwitchApi.history(cid)
      setHistory(Array.isArray(res?.data) ? res.data : [])
    } catch {
      setHistory([])
    } finally {
      setHistoryLoading(false)
    }
  }

  useEffect(() => {
    if (clusterId) fetchHistory(clusterId)
  }, [clusterId])

  const onClusterChange = (value: string) => {
    setClusterId(value)
    setSelectedInstance(undefined)
    setTargetRole(undefined)
    form.setFieldsValue({ instance_id: undefined, target_role: undefined })
  }

  const onSubmit = async () => {
    if (!clusterId || !selectedInstance || !targetRole || !selectedArch) {
      message.warning('\u8bf7\u5148\u9009\u62e9\u96c6\u7fa4\u3001\u76ee\u6807\u5b9e\u4f8b\u548c\u76ee\u6807\u89d2\u8272')
      return
    }
    const inst = instances.find((item) => item.id === selectedInstance)
    if (!inst || inst.cluster_id !== clusterId || instanceArch(inst) !== selectedArch) {
      message.error('\u76ee\u6807\u5b9e\u4f8b\u5fc5\u987b\u5c5e\u4e8e\u5f53\u524d\u96c6\u7fa4\u4e14\u67b6\u6784\u4e00\u81f4')
      return
    }
    setSubmitting(true)
    try {
      const res: any = await roleSwitchApi.switch({
        cluster_id: clusterId,
        instance_id: selectedInstance,
        target_role: targetRole,
      })
      const data = res?.data || res
      if (isFailedSwitchStatus(data?.status)) {
        message.error(data?.message || '\u89d2\u8272\u5207\u6362\u5931\u8d25')
      } else if (isCompletedSwitchStatus(data?.status)) {
        message.success(data?.message || '\u89d2\u8272\u5207\u6362\u6210\u529f')
      } else if (isSkippedSwitchStatus(data?.status)) {
        message.info(data?.message || '\u76ee\u6807\u5b9e\u4f8b\u5df2\u662f\u8be5\u89d2\u8272\uff0c\u672a\u6267\u884c\u5207\u6362')
      } else {
        message.info(data?.message || '\u89d2\u8272\u5207\u6362\u5df2\u8bb0\u5f55\uff0c\u8bf7\u67e5\u770b\u5386\u53f2\u72b6\u6001')
      }
      fetchHistory(clusterId)
    } catch (err: any) {
      message.error(err?.response?.data?.message || '\u89d2\u8272\u5207\u6362\u5931\u8d25')
    } finally {
      setSubmitting(false)
    }
  }
  const historyColumns: ColumnsType<SwitchResult> = [
    {
      title: '时间',
      key: 'time',
      render: (_, r) => {
        const time = r.occurred_at || r.completed_at || r.started_at
        return time ? new Date(time).toLocaleString() : '-'
      },
    },
    { title: '架构', dataIndex: 'cluster_type', key: 'cluster_type', render: (v) => v ? <Tag>{String(v).toUpperCase()}</Tag> : '-' },
    { title: '实例', dataIndex: 'instance_id', key: 'instance_id' },
    {
      title: '角色变化',
      key: 'role_change',
      render: (_, r) => (
        <Space>
          <Tag>{r.old_role || '-'}</Tag>
          <span>→</span>
          <Tag color="blue">{r.new_role || '-'}</Tag>
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (s: string) => (
        <Tag color={switchStatusColor(s)}>{s}</Tag>
      ),
    },
    { title: '消息', dataIndex: 'message', key: 'message' },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <Card title={<Space><RetweetOutlined /><span>集群内角色切换</span></Space>}>
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="角色切换只允许在同一个集群、同一种高可用架构内执行。选择集群后，架构会自动锁定。"
        />
        <Form form={form} layout="vertical" style={{ maxWidth: 760 }}>
          <Form.Item label="集群ID" required>
            <Select
              placeholder="选择集群"
              value={clusterId}
              onChange={onClusterChange}
              options={clusters.map((c) => ({ value: c.id, label: `${c.id} (${c.arch.toUpperCase()})` }))}
            />
          </Form.Item>
          <Form.Item label="架构类型">
            {selectedArch ? <Tag color="blue">{selectedArch.toUpperCase()}</Tag> : <span>-</span>}
          </Form.Item>
          <Form.Item label="目标实例" required>
            <Select
              placeholder="选择当前集群内实例"
              disabled={!clusterId}
              value={selectedInstance}
              onChange={(value) => {
                const next = clusterInstances.find((item) => item.id === value)
                setSelectedInstance(value)
                if (!next || next.status?.role === targetRole) {
                  setTargetRole(undefined)
                  form.setFieldsValue({ target_role: undefined })
                }
              }}
              options={clusterInstances.map((i) => ({
                value: i.id,
                label: `${i.name} (${i.connection?.host || i.host || '-'}:${i.connection?.port || i.port || '-'})`,
              }))}
            />
          </Form.Item>
          {selected && (
            <Descriptions size="small" bordered column={1} style={{ marginBottom: 16 }}>
              <Descriptions.Item label="当前角色">{selected.status?.role || '-'}</Descriptions.Item>
              <Descriptions.Item label="高可用架构">{selectedArch.toUpperCase()}</Descriptions.Item>
              <Descriptions.Item label="复制状态">{selected.status?.replication_status || selected.topology?.replication_mode || '-'}</Descriptions.Item>
            </Descriptions>
          )}
          <Form.Item label="目标角色" required>
            <Select
              placeholder="选择目标角色"
              disabled={!selectedArch}
              value={targetRole}
              onChange={setTargetRole}
              options={targetRoleOptions}
            />
          </Form.Item>
          <Form.Item>
            <Button
              type="primary"
              icon={<RetweetOutlined />}
              loading={submitting}
              onClick={onSubmit}
              disabled={!clusterId || !selectedInstance || !targetRole}
            >
              执行切换
            </Button>
          </Form.Item>
        </Form>
      </Card>

      <Card style={{ marginTop: 16 }} title={<Space><HistoryOutlined /><span>切换历史</span></Space>}>
        {!clusterId ? (
          <Result title="请先选择集群" subTitle="选择集群后查看角色切换历史" />
        ) : (
          <Table
            columns={historyColumns}
            dataSource={history}
            rowKey={(row) => row.id || `${row.instance_id}-${row.completed_at || row.started_at || row.message}`}
            loading={historyLoading}
            locale={{ emptyText: '暂无切换记录' }}
          />
        )}
      </Card>
    </div>
  )
}

export default RoleSwitch
