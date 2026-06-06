import React, { useEffect, useState } from 'react'
import { Card, Form, Select, Button, Space, message, Table, Tag, Result } from 'antd'
import { RetweetOutlined, HistoryOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { roleSwitchApi, instanceApi, type Instance } from '../services/api'

interface SwitchResult {
  cluster_id: string
  instance_id: string
  old_role: string
  new_role: string
  status: string
  message: string
  old_master_id: string
  new_master_id: string
  occurred_at: string
}

const TARGET_ROLES: Record<string, string[]> = {
  mha: ['master', 'slave'],
  mgr: ['primary', 'secondary'],
  pxc: ['primary', 'secondary'],
}

const RoleSwitch: React.FC = () => {
  const [instances, setInstances] = useState<Instance[]>([])
  const [clusterId, setClusterId] = useState<string | undefined>(undefined)
  const [archType, setArchType] = useState<string>('mha')
  const [targetRole, setTargetRole] = useState<string | undefined>(undefined)
  const [selectedInstance, setSelectedInstance] = useState<string | undefined>(undefined)
  const [submitting, setSubmitting] = useState(false)
  const [history, setHistory] = useState<SwitchResult[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)
  const [form] = Form.useForm()

  useEffect(() => {
    instanceApi.list(100, 0).then((res: any) => setInstances(res?.data || [])).catch(() => {})
  }, [])

  const fetchHistory = async (cid: string) => {
    setHistoryLoading(true)
    try {
      const res: any = await roleSwitchApi.history(cid)
      const list: SwitchResult[] = Array.isArray(res?.data) ? res.data : []
      setHistory(list)
    } catch {
      setHistory([])
    } finally {
      setHistoryLoading(false)
    }
  }

  useEffect(() => {
    if (clusterId) fetchHistory(clusterId)
  }, [clusterId])

  const clusterInstances = instances.filter((i) => i.cluster_id === clusterId)

  const onSubmit = async () => {
    if (!clusterId || !selectedInstance || !targetRole) {
      message.warning('请填写完整信息')
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
      message.success(`切换${data?.status === 'completed' ? '成功' : '已记录'}: ${data?.message || ''}`)
      fetchHistory(clusterId)
    } catch (err: any) {
      message.error(err?.response?.data?.message || '切换失败')
    } finally {
      setSubmitting(false)
    }
  }

  const historyColumns: ColumnsType<SwitchResult> = [
    { title: '时间', dataIndex: 'occurred_at', key: 'occurred_at' },
    { title: '实例', dataIndex: 'instance_id', key: 'instance_id' },
    {
      title: '角色变化',
      key: 'role_change',
      render: (_, r) => (
        <Space>
          <Tag>{r.old_role}</Tag>
          <span>→</span>
          <Tag color="blue">{r.new_role}</Tag>
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (s: string) => (
        <Tag color={s === 'completed' ? 'success' : s === 'failed' ? 'error' : 'default'}>{s}</Tag>
      ),
    },
    { title: '消息', dataIndex: 'message', key: 'message' },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title={
          <Space>
            <RetweetOutlined />
            <span>集群内角色切换</span>
          </Space>
        }
      >
        <Form form={form} layout="vertical" style={{ maxWidth: 720 }}>
          <Form.Item label="集群ID" required>
            <Select
              placeholder="选择集群"
              value={clusterId}
              onChange={(v: string) => {
                setClusterId(v)
                setSelectedInstance(undefined)
                form.setFieldsValue({ instance_id: undefined })
              }}
              options={Array.from(
                new Set(instances.map((i) => i.cluster_id).filter(Boolean)),
              ).map((c) => ({ value: c as string, label: c as string }))}
            />
          </Form.Item>
          <Form.Item label="架构类型" required>
            <Select
              value={archType}
              // P2: 之前从 MHA 切到 MGR, targetRole 仍是 "master" (MHA 角色),
              // 后端校验失败. 架构类型变化时清空 targetRole.
              onChange={(v) => { setArchType(v); setTargetRole(undefined) }}
              options={[
                { value: 'mha', label: 'MHA (master/slave)' },
                { value: 'mgr', label: 'MGR (primary/secondary)' },
                { value: 'pxc', label: 'PXC (primary/secondary)' },
              ]}
            />
          </Form.Item>
          <Form.Item label="目标实例" required>
            <Select
              placeholder="选择实例"
              disabled={!clusterId}
              value={selectedInstance}
              onChange={setSelectedInstance}
              options={clusterInstances.map((i) => ({ value: i.id, label: i.name }))}
            />
          </Form.Item>
          <Form.Item label="目标角色" required>
            <Select
              placeholder="选择目标角色"
              disabled={!archType}
              value={targetRole}
              onChange={setTargetRole}
              options={(TARGET_ROLES[archType] || []).map((r) => ({ value: r, label: r }))}
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

      <Card
        style={{ marginTop: 16 }}
        title={
          <Space>
            <HistoryOutlined />
            <span>切换历史</span>
          </Space>
        }
      >
        {!clusterId ? (
          <Result title="请先选择集群" subTitle="选择集群后查看角色切换历史" />
        ) : (
          <Table
            columns={historyColumns}
            dataSource={history}
            // P2: 之前 rowKey="occurred_at" 同秒会重复, React 报 duplicate key warn;
            // 改用后端返的 id (uuid).
            rowKey="id"
            loading={historyLoading}
            locale={{ emptyText: '暂无切换记录' }}
          />
        )}
      </Card>
    </div>
  )
}

export default RoleSwitch
