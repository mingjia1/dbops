import React, { useEffect, useState } from 'react'
import {
  Button, Card, Col, Descriptions, Form, Modal, Result, Row, Select, Space, Statistic, Table, Tabs, Tag, message,
} from 'antd'
import {
  AlertOutlined, HeartOutlined, SafetyCertificateOutlined, SwapOutlined, ThunderboltOutlined,
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { haApi, instanceApi, roleSwitchApi, type Instance } from '../services/api'
import {
  HAClusterStatus, PreflightResult,
  haNodeID, haNodeEndpoint, instanceEndpoint,
  isPrimaryRole, isReplicaRole,
  isFailedHAStatus, isCompletedHAStatus, isSkippedHAStatus, isPartialHAStatus,
  isMGRInstance, isPXCInstance, detectClusterArch,
} from '../services/haHelpers'
import { PreflightModal } from '../components/PreflightModal'
import { AutoFailoverModal } from '../components/AutoFailoverModal'
import { ManualSwitchModal } from '../components/ManualSwitchModal'

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

  const haErrorMessage = (err: any, fallback: string) =>
    err?.response?.data?.data?.error_message
    || err?.response?.data?.data?.message
    || err?.response?.data?.message
    || err?.message
    || fallback

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
      await refreshClusterData()
      message.error(haErrorMessage(err, '提交失败'))
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
          target_role: 'primary',
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
      await refreshClusterData()
      message.error(haErrorMessage(err, '提交失败'))
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

      <PreflightModal
        open={preflightOpen}
        loading={preflightLoading}
        result={preflight}
        onClose={() => setPreflightOpen(false)}
      />

      <AutoFailoverModal
        open={autoOpen}
        confirming={submitting}
        form={autoForm}
        forceMode={autoForce}
        onOk={submitAuto}
        onCancel={() => setAutoOpen(false)}
      />

      <ManualSwitchModal
        open={manualOpen}
        confirming={submitting}
        form={manualForm}
        slaveInstances={slaveInstances}
        clusterIsMGR={clusterIsMGR}
        clusterIsPXC={clusterIsPXC}
        forceMode={manualForce}
        preflightPass={preflightPass}
        onOk={submitManual}
        onCancel={() => setManualOpen(false)}
        onPreflight={openPreFlight}
      />
    </div>
  )
}

export default HAManage
