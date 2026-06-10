import React, { useEffect, useState } from 'react'
import {
  Card, Form, Input, Select, Switch, Button, Space, message, Table, Tag, Descriptions, Modal, Tabs, Result, Alert, Spin, Statistic, Row, Col,
} from 'antd'
import { HeartOutlined, ThunderboltOutlined, SwapOutlined, AlertOutlined, SafetyCertificateOutlined, ExclamationCircleOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { haApi, instanceApi, type Instance } from '../services/api'

interface HAClusterStatus {
  cluster_id: string
  master: any
  slaves: any[]
  history: any[]
}

const haNodeID = (node: any) => node?.instance_id || node?.id || '-'
const haNodeEndpoint = (node: any) => {
  if (!node) return '-'
  if (node.host && node.port) return `${node.host}:${node.port}`
  return node.host || '-'
}

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
interface PreflightResult {
  master_healthy: boolean
  slave_count: number
  healthy_slaves: number
  max_replication_lag: number
  gtid_consistent: boolean
  details: string[]
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

  useEffect(() => {
    instanceApi.list(100, 0).then((res: any) => setInstances(res?.data || [])).catch(() => {})
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

  const clusterInstances = instances.filter((i) => i.cluster_id === clusterId)
  const masterInstance = clusterInstances.find((i) => {
    const r = i.status?.role
    return r === 'master' || r === 'primary' || r === 'primary_master'
  })
  const slaveInstances = clusterInstances.filter((i) => {
    const r = i.status?.role
    return r === 'slave' || r === 'secondary' || r === 'replica'
  })

  const runPreFlightCheck = async (): Promise<PreflightResult> => {
    const details: string[] = []
    let masterHealthy = true
    let maxLag = 0
    let healthySlaves = 0
    let gtidConsistent = true

    if (masterInstance) {
      const h = masterInstance.status?.health_status
      if (h && h !== 'healthy' && h !== 'ok') {
        masterHealthy = false
        details.push(`主节点健康状态异常: ${h}`)
      } else {
        details.push(`主节点 ${masterInstance.name} 健康状态正常`)
      }
    } else {
      masterHealthy = false
      details.push('未检测到主节点, 请先确认集群状态')
    }

    slaveInstances.forEach((s) => {
      const lag = s.status?.seconds_behind_master ?? 0
      if (lag > maxLag) maxLag = lag
      const h = s.status?.health_status
      if (h === 'healthy' || h === 'ok' || !h) healthySlaves += 1
      if (lag > 30) details.push(`从节点 ${s.name} 复制延迟 ${lag}s, 过高`)
      const repl = s.status?.replication_status
      if (repl && repl !== 'running' && repl !== 'ok') {
        gtidConsistent = false
        details.push(`从节点 ${s.name} 复制状态: ${repl}`)
      }
    })

    if (slaveInstances.length === 0) {
      details.push('警告: 无可用从节点, 故障转移后将无新主可用')
    }

    return {
      master_healthy: masterHealthy,
      slave_count: slaveInstances.length,
      healthy_slaves: healthySlaves,
      max_replication_lag: maxLag,
      gtid_consistent: gtidConsistent,
      details,
    }
  }

  const openPreFlight = async () => {
    setPreflightOpen(true)
    setPreflightLoading(true)
    setPreflight(null)
    try {
      const r = await runPreFlightCheck()
      setPreflight(r)
    } finally {
      setPreflightLoading(false)
    }
  }

  const submitAuto = async () => {
    try {
      const values = await autoForm.validateFields()
      if (!values.confirm_impact) {
        message.warning('\u8bf7\u786e\u8ba4\u5df2\u4e86\u89e3\u64cd\u4f5c\u5f71\u54cd')
        return
      }
      setSubmitting(true)
      const res: any = await haApi.autoFailover({ cluster_id: clusterId, ...values, require_restart: values.require_restart || false })
      const data = res?.data || res
      if (isFailedHAStatus(data?.status)) {
        message.error(data?.error_message || data?.message || '\u81ea\u52a8\u6545\u969c\u8f6c\u79fb\u5931\u8d25')
      } else if (isSkippedHAStatus(data?.status)) {
        message.info(data?.error_message || data?.message || '\u672a\u89e6\u53d1\u81ea\u52a8\u6545\u969c\u8f6c\u79fb')
      } else if (isPartialHAStatus(data?.status)) {
        message.warning(data?.error_message || data?.message || '\u6545\u969c\u8f6c\u79fb\u90e8\u5206\u5b8c\u6210')
      } else if (isCompletedHAStatus(data?.status)) {
        message.success(data?.message || '\u81ea\u52a8\u6545\u969c\u8f6c\u79fb\u5b8c\u6210')
      } else {
        message.info(data?.message || '\u81ea\u52a8\u6545\u969c\u8f6c\u79fb\u5df2\u63d0\u4ea4')
      }
      setAutoOpen(false)
      autoForm.resetFields()
      if (clusterId) fetchStatus(clusterId)
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '\u63d0\u4ea4\u5931\u8d25')
    } finally {
      setSubmitting(false)
    }
  }
  const submitManual = async () => {
    try {
      const values = await manualForm.validateFields()
      if (!values.confirm_impact) {
        message.warning('\u8bf7\u786e\u8ba4\u5df2\u4e86\u89e3\u64cd\u4f5c\u5f71\u54cd')
        return
      }
      setSubmitting(true)
      const res: any = await haApi.manualSwitch({ cluster_id: clusterId, ...values })
      const data = res?.data || res
      if (isFailedHAStatus(data?.status)) {
        message.error(data?.error_message || data?.message || '\u624b\u52a8\u5207\u6362\u5931\u8d25')
      } else if (isSkippedHAStatus(data?.status)) {
        message.info(data?.error_message || data?.message || '\u624b\u52a8\u5207\u6362\u672a\u6267\u884c')
      } else if (isPartialHAStatus(data?.status)) {
        message.warning(data?.error_message || data?.message || '\u624b\u52a8\u5207\u6362\u90e8\u5206\u5b8c\u6210')
      } else if (isCompletedHAStatus(data?.status)) {
        message.success(data?.message || '\u624b\u52a8\u5207\u6362\u5b8c\u6210')
      } else {
        message.info(data?.message || '\u624b\u52a8\u5207\u6362\u5df2\u63d0\u4ea4')
      }
      setManualOpen(false)
      manualForm.resetFields()
      if (clusterId) fetchStatus(clusterId)
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '\u63d0\u4ea4\u5931\u8d25')
    } finally {
      setSubmitting(false)
    }
  }
  const runBatchHealth = async () => {
    const scopedInstances = clusterId ? instances.filter((i) => i.cluster_id === clusterId) : instances
    const ids = scopedInstances.filter((i) => i.host_id).map((i) => i.id)
    if (ids.length === 0) {
      message.warning('\u6ca1\u6709\u53ef\u68c0\u6d4b\u7684\u5b9e\u4f8b')
      return
    }
    try {
      const res: any = await haApi.healthCheck({ instance_ids: ids })
      const rows = Array.isArray(res?.data) ? res.data : []
      const failed = rows.filter((row: any) => !row?.is_healthy || isFailedHAStatus(row?.status) || row?.status === 'unhealthy')
      if (failed.length > 0) {
        Modal.warning({
          title: `\u5065\u5eb7\u68c0\u67e5\u90e8\u5206\u5931\u8d25\uff1a\u6b63\u5e38 ${rows.length - failed.length} \u4e2a\uff0c\u5f02\u5e38 ${failed.length} \u4e2a`,
          content: (
            <div style={{ maxHeight: 260, overflow: 'auto', whiteSpace: 'pre-wrap' }}>
              {failed.map((row: any) => `${row.instance_id || '-'}: ${row.error_message || row.status || 'unhealthy'}`).join('\n')}
            </div>
          ),
        })
      } else {
        message.success(`\u5065\u5eb7\u68c0\u67e5\u5b8c\u6210\uff1a${rows.length || ids.length} \u4e2a\u5b9e\u4f8b\u6b63\u5e38`)
      }
    } catch (err: any) {
      message.error(err?.response?.data?.message || '\u5065\u5eb7\u68c0\u67e5\u5931\u8d25')
    }
  }
  const slaveColumns: ColumnsType<any> = [
    { title: '实例ID', key: 'id', render: (_, row) => haNodeID(row) },
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

  const preflightPass = preflight
    && preflight.master_healthy
    && preflight.healthy_slaves > 0
    && preflight.max_replication_lag <= 30
    && preflight.gtid_consistent

  return (
    <div style={{ padding: '24px' }}>
      <Alert
        type="error"
        showIcon
        icon={<ExclamationCircleOutlined />}
        style={{ marginBottom: 16 }}
        message="危险操作警告"
        description="故障转移会导致短暂的服务中断, 并可能造成数据丢失。在执行前请确保已完成备份, 并知会相关业务方。所有操作将记录在审计日志中。"
      />
      <Card
        title={
          <Space>
            <HeartOutlined />
            <span>高可用管理</span>
          </Space>
        }
        extra={
          <Space>
            <Select
              placeholder="选择集群"
              style={{ width: 240 }}
              value={clusterId}
              onChange={(v) => {
                setClusterId(v)
                setPreflight(null)
              }}
              options={Array.from(
                new Set(instances.map((i) => i.cluster_id).filter(Boolean)),
              ).map((c) => ({ value: c as string, label: c as string }))}
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
        }
      >
        {clusterId && (
          <Row gutter={16} style={{ marginBottom: 16 }}>
            <Col span={6}>
              <Card size="small"><Statistic title="主节点" value={masterInstance?.name || '无主'} valueStyle={{ fontSize: 14, color: masterInstance ? '#3f8600' : '#cf1322' }} /></Card>
            </Col>
            <Col span={6}>
              <Card size="small"><Statistic title="从节点数" value={slaveInstances.length} /></Card>
            </Col>
            <Col span={6}>
              <Card size="small"><Statistic title="最大复制延迟(s)" value={Math.max(0, ...slaveInstances.map((s) => s.status?.seconds_behind_master ?? 0))} valueStyle={{ color: '#fa8c16' }} /></Card>
            </Col>
            <Col span={6}>
              <Card size="small"><Statistic title="切换历史" value={status?.history?.length || 0} /></Card>
            </Col>
          </Row>
        )}
        {!clusterId ? (
          <Result
            title="请先选择集群"
            subTitle="在右上角的集群下拉框中选择一个集群ID以查看状态和执行切换"
            icon={<HeartOutlined />}
          />
        ) : !status ? (
          <Result status="warning" title="暂无集群状态" subTitle="请确认集群ID有效或稍后重试" />
        ) : (
          <Tabs
            items={[
              {
                key: 'overview',
                label: '状态总览',
                children: (
                  <Descriptions bordered column={2}>
                    <Descriptions.Item label="集群ID">{status.cluster_id}</Descriptions.Item>
                    <Descriptions.Item label="主节点">
                      {status.master ? (
                        <Space>
                          <Tag color="blue">{haNodeID(status.master)}</Tag>
                          <span>{haNodeEndpoint(status.master)}</span>
                        </Space>
                      ) : '-'}
                    </Descriptions.Item>
                    <Descriptions.Item label="从节点数">{status.slaves?.length || 0}</Descriptions.Item>
                    <Descriptions.Item label="历史切换">{status.history?.length || 0}</Descriptions.Item>
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
        width={700}
      >
        {preflightLoading ? (
          <div style={{ textAlign: 'center', padding: 30 }}><Spin /></div>
        ) : preflight ? (
          <>
            <Row gutter={16} style={{ marginBottom: 16 }}>
              <Col span={6}>
                <Statistic
                  title="主节点健康"
                  value={preflight.master_healthy ? '是' : '否'}
                  valueStyle={{ color: preflight.master_healthy ? '#3f8600' : '#cf1322' }}
                />
              </Col>
              <Col span={6}>
                <Statistic
                  title="健康从节点"
                  value={`${preflight.healthy_slaves} / ${preflight.slave_count}`}
                />
              </Col>
              <Col span={6}>
                <Statistic
                  title="最大复制延迟"
                  value={preflight.max_replication_lag}
                  suffix="s"
                  valueStyle={{ color: preflight.max_replication_lag > 30 ? '#cf1322' : '#3f8600' }}
                />
              </Col>
              <Col span={6}>
                <Statistic
                  title="GTID 一致"
                  value={preflight.gtid_consistent ? '是' : '否'}
                  valueStyle={{ color: preflight.gtid_consistent ? '#3f8600' : '#cf1322' }}
                />
              </Col>
            </Row>
            <Alert
              type={preflightPass ? 'success' : 'error'}
              showIcon
              message={preflightPass ? '检查通过, 可执行切换' : '存在风险, 不建议执行切换'}
              description={
                <ul style={{ marginBottom: 0, paddingLeft: 18 }}>
                  {preflight.details.map((d, i) => (<li key={i}>{d}</li>))}
                </ul>
              }
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
            message="该操作将自动选出新主并执行切换, 会导致短暂不可用"
            description="建议先执行 Pre-flight 检查, 并在业务低峰期执行"
            style={{ marginBottom: 12 }}
          />
          <Form.Item name="trigger_instance_id" label="触发实例ID" rules={[{ required: true }]}>
            <Input placeholder="例如: inst-001" />
          </Form.Item>
          <Form.Item name="reason" label="故障原因" rules={[{ required: true }]}>
            <Input placeholder="例如: 主库宕机" />
          </Form.Item>
          <Form.Item name="force" label="强制执行 (跳过数据一致性校验)" valuePropName="checked">
            <Switch checkedChildren="强制" unCheckedChildren="安全模式" />
          </Form.Item>
          {autoForce && (
            <Alert
              type="warning"
              showIcon
              message="强制模式可能造成数据丢失, 请确认"
            />
          )}
          <Form.Item name="confirm_impact" valuePropName="checked" rules={[{ required: true, message: '请确认' }]}>
            <Switch checkedChildren="已确认影响" unCheckedChildren="未确认" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="手动主从切换"
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
            message="该操作将变更主从关系, 会导致短暂不可用"
            description="建议先执行 Pre-flight 检查, 并在业务低峰期执行"
            style={{ marginBottom: 12 }}
          />
          <Form.Item name="new_master_id" label="新主实例ID" rules={[{ required: true }]}>
            <Select
              placeholder="选择新主实例"
              options={slaveInstances.map((s) => ({ value: s.id, label: `${s.name} (延迟: ${s.status?.seconds_behind_master ?? '-'}s)` }))}
            />
          </Form.Item>
          <Form.Item name="reason" label="切换原因" rules={[{ required: true }]}>
            <Input placeholder="例如: 计划内维护" />
          </Form.Item>
          <Form.Item name="force" label="强制执行 (跳过数据一致性校验)" valuePropName="checked">
            <Switch checkedChildren="强制" unCheckedChildren="安全模式" />
          </Form.Item>
          {manualForce && (
            <Alert
              type="warning"
              showIcon
              message="强制模式可能造成数据丢失, 请确认"
            />
          )}
          <Form.Item name="confirm_impact" valuePropName="checked" rules={[{ required: true, message: '请确认' }]}>
            <Switch checkedChildren="已确认影响" unCheckedChildren="未确认" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default HAManage
