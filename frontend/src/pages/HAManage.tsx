import React, { useEffect, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Input,
  Modal,
  Result,
  Row,
  Select,
  Space,
  Spin,
  Statistic,
  Switch,
  Table,
  Tabs,
  Tag,
  message,
} from 'antd'
import {
  AlertOutlined,
  HeartOutlined,
  SafetyCertificateOutlined,
  SwapOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { haApi, instanceApi, roleSwitchApi, type Instance } from '../services/api'

interface HAClusterStatus {
  cluster_id: string
  master: any
  slaves: any[]
  history: any[]
}

interface PreflightResult {
  cluster_id: string
  target_master_id?: string
  current_master_id: string
  current_master_healthy: boolean
  healthy_slave_count: number
  slave_count: number
  max_replication_lag: number
  gtid_consistent: boolean
  topology_consistent: boolean
  real_primary_id?: string
  platform_primary_id?: string
  pass: boolean
  blocking_reasons?: string[]
  warnings?: string[]
}

const haNodeID = (node: any) => node?.instance_id || node?.id || '-'

const haNodeEndpoint = (node: any) => {
  if (!node) return '-'
  if (node.host && node.port) return `${node.host}:${node.port}`
  return node.host || '-'
}

const instanceEndpoint = (instance?: Instance) => {
  if (!instance) return '-'
  const host = instance.host || instance.connection?.host
  const port = instance.port || instance.connection?.port
  if (host && port) return `${host}:${port}`
  return host || '-'
}

const normalizeRole = (role?: string) => (role || '').toLowerCase()
const isPrimaryRole = (role?: string) => ['master', 'primary', 'primary_master', 'bootstrap'].includes(normalizeRole(role))
const isReplicaRole = (role?: string) => ['slave', 'secondary', 'replica', 'donor', 'joiner'].includes(normalizeRole(role))

const isFailedHAStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(normalized)
}

const isCompletedHAStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['completed', 'success', 'succeeded', 'ok'].includes(normalized)
}

const isSkippedHAStatus = (status?: string) => (status || '').toLowerCase() === 'skipped'
const isPartialHAStatus = (status?: string) => (status || '').toLowerCase() === 'partial_success'

const isMGRInstance = (instance: Instance) => {
  const mode = (instance.topology?.replication_mode || '').toLowerCase()
  const repl = (instance.status?.replication_status || '').toLowerCase()
  return mode === 'mgr' || repl === 'mgr' || mode.includes('group_replication') || repl.includes('group_replication')
}

const isPXCInstance = (instance: Instance) => {
  const mode = (instance.topology?.replication_mode || '').toLowerCase()
  const repl = (instance.status?.replication_status || '').toLowerCase()
  return mode === 'pxc' || repl === 'pxc' || mode.includes('galera') || repl.includes('galera') || mode.includes('wsrep') || repl.includes('wsrep')
}

const detectClusterArch = (instances: Instance[]): 'ha' | 'mha' | 'mgr' | 'pxc' | '' => {
  if (instances.some(isMGRInstance)) return 'mgr'
  if (instances.some(isPXCInstance)) return 'pxc'
  const mode = (instances[0]?.topology?.replication_mode || '').toLowerCase()
  if (mode === 'mha') return 'mha'
  if (instances.length > 0) return 'ha'
  return ''
}

const HAManage: React.FC = () => {
  const [instances, setInstances] = useState<Instance[]>([])
  const [clusterId, setClusterId] = useState<string | undefined>(undefined)
  const [status, setStatus] = useState<HAClusterStatus | null>(null)
  const [autoOpen, setAutoOpen] = useState(false)
  const [manualOpen, setManualOpen] = useState(false)
  const [preflightOpen, setPreflightOpen] = useState(false)
  const [preflight, setPreflight] = useState<PreflightResult | null>(null)
  const [preflightLoading, setPreflightLoading] = useState(false)
  const [autoForm] = Form.useForm()
  const [manualForm] = Form.useForm()
  const [submitting, setSubmitting] = useState(false)
  const autoForce = Form.useWatch('force', autoForm)
  const manualForce = Form.useWatch('force', manualForm)
  const selectedTargetMasterID = Form.useWatch('new_master_id', manualForm)

  const loadInstances = async () => {
    const res: any = await instanceApi.list(100, 0)
    setInstances(res?.data || [])
  }

  useEffect(() => {
    loadInstances().catch(() => { /* soft fallback: instance list unavailable */ })
  }, [])

  const fetchStatus = async (cid: string) => {
    try {
      const res: any = await haApi.getStatus(cid, 20)
      setStatus(res?.data || null)
    } catch {
      setStatus(null)
    }
  }

  useEffect(() => {
    if (clusterId) fetchStatus(clusterId)
  }, [clusterId])

  useEffect(() => {
    setPreflight(null)
  }, [selectedTargetMasterID, manualForce])

  const clusterInstances = instances.filter((i) => i.cluster_id === clusterId)
  const masterInstance = clusterInstances.find((i) => isPrimaryRole(i.status?.role))
  const slaveInstances = clusterInstances.filter((i) => isReplicaRole(i.status?.role))
  const clusterArch = detectClusterArch(clusterInstances)
  const clusterIsMGR = clusterArch === 'mgr'
  const clusterIsPXC = clusterArch === 'pxc'
  const clusterSupportsRoleSwitch = clusterIsMGR || clusterIsPXC
  const preflightPass = !!preflight?.pass

  const refreshClusterData = async () => {
    await loadInstances().catch(() => { /* soft fallback: instance list unavailable */ })
    if (clusterId) await fetchStatus(clusterId)
  }

  const runPreFlightCheck = async (): Promise<PreflightResult> => {
    if (!clusterId) throw new Error('请先选择集群')
    const res: any = await haApi.preflight({
      cluster_id: clusterId,
      target_master_id: manualForm.getFieldValue('new_master_id') || '',
      force: manualForm.getFieldValue('force') || false,
    })
    return res?.data || res
  }

  const openPreFlight = async () => {
    setPreflightOpen(true)
    setPreflightLoading(true)
    setPreflight(null)
    try {
      const result = await runPreFlightCheck()
      setPreflight(result)
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || 'Pre-flight 检查失败')
    } finally {
      setPreflightLoading(false)
    }
  }

  const submitAuto = async () => {
    try {
      const values = await autoForm.validateFields()
      if (!values.confirm_impact) {
        message.warning('请确认已了解操作影响')
        return
      }
      setSubmitting(true)
      const res: any = await haApi.autoFailover({
        cluster_id: clusterId,
        reason: values.reason,
        force: values.force || false,
        require_restart: values.require_restart || false,
      })
      const data = res?.data || res
      if (isFailedHAStatus(data?.status)) {
        message.error(data?.error_message || data?.message || '自动故障转移失败')
      } else if (isSkippedHAStatus(data?.status)) {
        message.info(data?.error_message || data?.message || '未触发自动故障转移')
      } else if (isPartialHAStatus(data?.status)) {
        message.warning(data?.error_message || data?.message || '故障转移部分完成')
      } else if (isCompletedHAStatus(data?.status)) {
        message.success(data?.message || '自动故障转移完成')
      } else {
        message.info(data?.message || '自动故障转移已提交')
      }
      setAutoOpen(false)
      autoForm.resetFields()
      await refreshClusterData()
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '提交失败')
    } finally {
      setSubmitting(false)
    }
  }

  const submitManual = async () => {
    try {
      const values = await manualForm.validateFields()
      if (!values.confirm_impact) {
        message.warning('请确认已了解操作影响')
        return
      }
      if (!values.force && !preflightPass) {
        message.warning('非强制模式需要先通过 Pre-flight 检查')
        await openPreFlight()
        return
      }
      const target = slaveInstances.find((i) => i.id === values.new_master_id)
      if (!target) {
        message.error('请选择一个非主节点作为新主')
        return
      }

      setSubmitting(true)
      const res: any = clusterSupportsRoleSwitch
        ? await roleSwitchApi.switch({
          cluster_id: clusterId as string,
          instance_id: values.new_master_id,
          target_role: clusterIsPXC ? 'secondary' : 'primary',
          old_master_id: masterInstance?.id,
        })
        : await haApi.manualSwitch({
          cluster_id: clusterId,
          new_master_id: values.new_master_id,
          reason: values.reason,
          force: values.force || false,
          confirm_impact: values.confirm_impact,
        })
      const data = res?.data || res
      if (isFailedHAStatus(data?.status)) {
        message.error(data?.error_message || data?.message || '手动切换失败')
      } else if (isSkippedHAStatus(data?.status)) {
        message.info(data?.error_message || data?.message || '手动切换未执行')
      } else if (isPartialHAStatus(data?.status)) {
        message.warning(data?.error_message || data?.message || '手动切换部分完成')
      } else if (isCompletedHAStatus(data?.status) || data?.success !== false) {
        message.success(data?.message || '手动切换完成')
      } else {
        message.info(data?.message || '手动切换已提交')
      }
      setManualOpen(false)
      manualForm.resetFields()
      setPreflight(null)
      await refreshClusterData()
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '提交失败')
    } finally {
      setSubmitting(false)
    }
  }

  const runBatchHealth = async () => {
    const scopedInstances = clusterId ? instances.filter((i) => i.cluster_id === clusterId) : instances
    const ids = scopedInstances.filter((i) => i.host_id).map((i) => i.id)
    if (ids.length === 0) {
      message.warning('没有可检测的实例')
      return
    }
    try {
      const res: any = await haApi.healthCheck({ instance_ids: ids })
      const rows = Array.isArray(res?.data) ? res.data : []
      const failed = rows.filter((row: any) => !row?.is_healthy || isFailedHAStatus(row?.status) || row?.status === 'unhealthy')
      if (failed.length > 0) {
        Modal.warning({
          title: `健康检查部分失败：正常 ${rows.length - failed.length} 个，异常 ${failed.length} 个`,
          content: (
            <div style={{ maxHeight: 260, overflow: 'auto', whiteSpace: 'pre-wrap' }}>
              {failed.map((row: any) => `${row.instance_id || '-'}: ${row.error_message || row.status || 'unhealthy'}`).join('\n')}
            </div>
          ),
        })
      } else {
        message.success(`健康检查完成：${rows.length || ids.length} 个实例正常`)
      }
    } catch (err: any) {
      message.error(err?.response?.data?.message || '健康检查失败')
    }
  }

  const slaveColumns: ColumnsType<any> = [
    { title: '实例 ID', key: 'id', render: (_, row) => haNodeID(row) },
    { title: '地址', key: 'endpoint', render: (_, row) => haNodeEndpoint(row) },
    {
      title: '健康',
      key: 'status',
      render: (_, row) => (
        <Tag color={row?.is_healthy ? 'success' : 'error'}>{row?.is_healthy ? 'healthy' : 'unhealthy'}</Tag>
      ),
    },
    { title: '角色', dataIndex: 'role', key: 'role', render: (role: string) => <Tag>{role || '-'}</Tag> },
  ]

  const historyColumns: ColumnsType<any> = [
    { title: '时间', dataIndex: 'created_at', key: 'created_at' },
    { title: '类型', dataIndex: 'type', key: 'type' },
    { title: '源', dataIndex: 'from', key: 'from' },
    { title: '目标', dataIndex: 'to', key: 'to' },
    {
      title: '结果',
      dataIndex: 'result',
      key: 'result',
      render: (r: string) => <Tag color={r === 'success' ? 'success' : 'error'}>{r}</Tag>,
    },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title={(
          <Space>
            <HeartOutlined />
            <span>高可用管理</span>
          </Space>
        )}
        extra={(
          <Space wrap>
            <Select
              placeholder="选择集群"
              style={{ width: 240 }}
              value={clusterId}
              onChange={(value) => {
                setClusterId(value)
                setPreflight(null)
              }}
              options={Array.from(new Set(instances.map((i) => i.cluster_id).filter(Boolean)))
                .map((c) => ({ value: c as string, label: c as string }))}
            />
            <Button icon={<SafetyCertificateOutlined />} onClick={openPreFlight} disabled={!clusterId}>
              Pre-flight 检查
            </Button>
            <Button icon={<ThunderboltOutlined />} onClick={runBatchHealth}>
              批量健康检查
            </Button>
            <Button type="primary" danger icon={<AlertOutlined />} onClick={() => setAutoOpen(true)} disabled={!clusterId}>
              自动故障转移
            </Button>
            <Button icon={<SwapOutlined />} onClick={() => setManualOpen(true)} disabled={!clusterId}>
              手动切换
            </Button>
          </Space>
        )}
      >
        {clusterId && (
          <Row gutter={16} style={{ marginBottom: 16 }}>
            <Col span={6}>
              <Card size="small">
                <Statistic
                  title={clusterIsPXC || clusterIsMGR ? '主节点' : '主节点'}
                  value={masterInstance ? instanceEndpoint(masterInstance) : '无主'}
                  valueStyle={{ fontSize: 14, color: masterInstance ? '#3f8600' : '#cf1322' }}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card size="small"><Statistic title={clusterIsPXC || clusterIsMGR ? '非主节点数' : '从节点数'} value={slaveInstances.length} /></Card>
            </Col>
            <Col span={6}>
              <Card size="small">
                <Statistic
                  title="最大复制延迟(s)"
                  value={Math.max(0, ...slaveInstances.map((s) => s.status?.seconds_behind_master ?? 0))}
                  valueStyle={{ color: '#fa8c16' }}
                />
              </Card>
            </Col>
            <Col span={6}>
              <Card size="small"><Statistic title="切换历史" value={status?.history?.length || 0} /></Card>
            </Col>
          </Row>
        )}
        {!clusterId ? (
          <Result title="请先选择集群" subTitle="选择集群后可查看状态、执行健康检查和切换操作" icon={<HeartOutlined />} />
        ) : !status ? (
          <Result status="warning" title="暂无集群状态" subTitle="请确认集群 ID 有效或稍后重试" />
        ) : (
          <Tabs
            items={[
              {
                key: 'overview',
                label: '状态总览',
                children: (
                  <Descriptions bordered column={2}>
                    <Descriptions.Item label="集群 ID">{status.cluster_id}</Descriptions.Item>
                    <Descriptions.Item label="主节点">
                      {status.master ? (
                        <Space>
                          <Tag color="blue">{haNodeID(status.master)}</Tag>
                          <span>{haNodeEndpoint(status.master)}</span>
                        </Space>
                      ) : '-'}
                    </Descriptions.Item>
                    <Descriptions.Item label="从节点数">{status.slaves?.length || 0}</Descriptions.Item>
                    <Descriptions.Item label="切换历史">{status.history?.length || 0}</Descriptions.Item>
                  </Descriptions>
                ),
              },
              {
                key: 'slaves',
                label: '从节点',
                children: <Table columns={slaveColumns} dataSource={status.slaves || []} rowKey={(row) => haNodeID(row)} />,
              },
              {
                key: 'history',
                label: '切换历史',
                children: <Table columns={historyColumns} dataSource={status.history || []} rowKey="created_at" />,
              },
            ]}
          />
        )}
      </Card>

      <Modal
        title="Pre-flight 检查"
        open={preflightOpen}
        onCancel={() => setPreflightOpen(false)}
        footer={<Button onClick={() => setPreflightOpen(false)}>关闭</Button>}
        width={760}
      >
        {preflightLoading ? (
          <div style={{ textAlign: 'center', padding: 30 }}><Spin /></div>
        ) : preflight ? (
          <>
            <Row gutter={16} style={{ marginBottom: 16 }}>
              <Col span={6}>
                <Statistic title="主节点健康" value={preflight.current_master_healthy ? '是' : '否'} valueStyle={{ color: preflight.current_master_healthy ? '#3f8600' : '#cf1322' }} />
              </Col>
              <Col span={6}>
                <Statistic title="健康从节点" value={`${preflight.healthy_slave_count} / ${preflight.slave_count}`} />
              </Col>
              <Col span={6}>
                <Statistic title="最大复制延迟" value={preflight.max_replication_lag} suffix="s" valueStyle={{ color: preflight.max_replication_lag > 30 ? '#cf1322' : '#3f8600' }} />
              </Col>
              <Col span={6}>
                <Statistic title="GTID 一致" value={preflight.gtid_consistent ? '是' : '否'} valueStyle={{ color: preflight.gtid_consistent ? '#3f8600' : '#cf1322' }} />
              </Col>
            </Row>
            <Descriptions bordered size="small" column={2} style={{ marginBottom: 16 }}>
              <Descriptions.Item label="平台主节点">{preflight.platform_primary_id || preflight.current_master_id || '-'}</Descriptions.Item>
              <Descriptions.Item label="真实主节点">{preflight.real_primary_id || '-'}</Descriptions.Item>
              <Descriptions.Item label="目标新主">{preflight.target_master_id || '-'}</Descriptions.Item>
              <Descriptions.Item label="拓扑一致">{preflight.topology_consistent ? '是' : '否'}</Descriptions.Item>
            </Descriptions>
            <Alert
              type={preflightPass ? 'success' : 'error'}
              showIcon
              message={preflightPass ? '检查通过，可以在非强制模式下切换' : '检查未通过，非强制模式不允许切换'}
              description={(
                <div>
                  {(preflight.blocking_reasons?.length || 0) > 0 && (
                    <ul style={{ marginBottom: 8, paddingLeft: 18 }}>
                      {preflight.blocking_reasons?.map((item, index) => <li key={`block-${index}`}>{item}</li>)}
                    </ul>
                  )}
                  {(preflight.warnings?.length || 0) > 0 && (
                    <ul style={{ marginBottom: 0, paddingLeft: 18 }}>
                      {preflight.warnings?.map((item, index) => <li key={`warn-${index}`}>{item}</li>)}
                    </ul>
                  )}
                </div>
              )}
            />
          </>
        ) : null}
      </Modal>

      <Modal
        title="自动故障转移"
        open={autoOpen}
        onCancel={() => setAutoOpen(false)}
        onOk={submitAuto}
        confirmLoading={submitting}
        okText="确认执行"
        cancelText="取消"
        okButtonProps={{ danger: true }}
      >
        <Form form={autoForm} layout="vertical">
          <Alert
            type="error"
            showIcon
            message="自动故障转移只在系统确认主节点故障后执行"
            description="自动故障转移不需要输入触发实例 ID，后端会根据当前集群主节点和故障确认状态决定是否切换。"
            style={{ marginBottom: 12 }}
          />
          <Form.Item name="reason" label="故障原因" rules={[{ required: true, message: '请输入故障原因' }]}>
            <Input placeholder="例如：主库宕机" />
          </Form.Item>
          <Form.Item name="force" label="强制执行" valuePropName="checked">
            <Switch checkedChildren="强制" unCheckedChildren="安全模式" />
          </Form.Item>
          {autoForce && <Alert type="warning" showIcon message="强制模式可能绕过保护检查，请确认业务和数据风险。" style={{ marginBottom: 12 }} />}
          <Form.Item name="confirm_impact" valuePropName="checked" rules={[{ required: true, message: '请确认操作影响' }]}>
            <Switch checkedChildren="已确认影响" unCheckedChildren="未确认" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={clusterIsPXC ? 'PXC 集群角色切换' : clusterIsMGR ? 'MGR 集群角色切换' : '手动主从切换'}
        open={manualOpen}
        onCancel={() => setManualOpen(false)}
        onOk={submitManual}
        confirmLoading={submitting}
        okText="执行"
        cancelText="取消"
        okButtonProps={{ danger: true }}
      >
        <Form form={manualForm} layout="vertical">
          <Alert
            type="error"
            showIcon
            message={clusterIsPXC ? 'PXC 集群将通过角色切换接口完成主从切换' : clusterIsMGR ? 'MGR 集群将通过角色切换接口完成主从切换' : '手动切换需要选择一个非主节点作为新主'}
            description={clusterIsPXC
              ? '选择一个 secondary 节点，系统将通过角色切换接口将其提升为 primary。'
              : clusterIsMGR
                ? '选择一个 secondary 节点，系统将通过角色切换接口将其提升为 primary。'
                : '未选择强制模式时，Pre-flight 通过后即可切换。'}
            style={{ marginBottom: 12 }}
          />
          <Form.Item name="new_master_id" label={clusterIsPXC ? '目标节点' : '新主实例'} rules={[{ required: true, message: clusterIsPXC ? '请选择目标节点' : '请选择新主实例' }]}>
            <Select
              placeholder={clusterIsPXC ? '选择 secondary 节点' : '选择非主节点'}
              onChange={() => setPreflight(null)}
              options={slaveInstances.map((instance) => ({
                value: instance.id,
                label: `${instance.name} (${instanceEndpoint(instance)}, 延迟: ${instance.status?.seconds_behind_master ?? '-'}s)`,
              }))}
            />
          </Form.Item>
          <Form.Item name="reason" label="切换原因" rules={[{ required: true, message: '请输入切换原因' }]}>
            <Input placeholder="例如：计划内维护" />
          </Form.Item>
          <Form.Item name="force" label="强制执行" valuePropName="checked">
            <Switch checkedChildren="强制" unCheckedChildren="安全模式" />
          </Form.Item>
          {manualForce ? (
            <Alert type="warning" showIcon message="强制模式会绕过 Pre-flight 阻断项，请确认数据风险。" style={{ marginBottom: 12 }} />
          ) : (
            <Alert
              type={preflightPass ? 'success' : 'info'}
              showIcon
              message={preflightPass ? 'Pre-flight 已通过' : '非强制模式需要先通过 Pre-flight 检查'}
              action={<Button size="small" onClick={openPreFlight}>检查</Button>}
              style={{ marginBottom: 12 }}
            />
          )}
          <Form.Item name="confirm_impact" valuePropName="checked" rules={[{ required: true, message: '请确认操作影响' }]}>
            <Switch checkedChildren="已确认影响" unCheckedChildren="未确认" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default HAManage
