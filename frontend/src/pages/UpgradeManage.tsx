import React, { useEffect, useMemo, useState } from 'react'
import {
  Button, Card, Descriptions, Form, Modal, Progress, Select, Space, Spin, Table, Tag, Typography, message,
} from 'antd'
import {
  CheckCircleOutlined, FileTextOutlined, PlayCircleOutlined, ReloadOutlined, RollbackOutlined,
} from '@ant-design/icons'
import { instanceApi, upgradeApi, versionApi, type Instance, type InstanceVersion, type VersionEntry } from '../services/api'
import {
  ActiveUpgrade, UpgradeHistory,
  activeUpgradeStatuses, terminalUpgradeStatuses,
  isCompletedUpgradeStatus, isFailedUpgradeStatus,
  upgradeStagesFor, inferStepIndex, currentUpgradeStage,
} from '../services/upgradeHelpers'
import UpgradePlanModal from '../components/UpgradePlanModal'
import CompatCheckModal from '../components/CompatCheckModal'
import ExecuteUpgradeModal from '../components/ExecuteUpgradeModal'
import UpgradeReportModal from '../components/UpgradeReportModal'
import UpgradeProgressCard from '../components/UpgradeProgressCard'

const { Title } = Typography

const UpgradeManage: React.FC = () => {
  const [history, setHistory] = useState<UpgradeHistory[]>([])
  const [instances, setInstances] = useState<Instance[]>([])
  const [versions, setVersions] = useState<VersionEntry[]>([])
  const [versionsLoading, setVersionsLoading] = useState(false)
  const [detectingVersions, setDetectingVersions] = useState<Record<string, boolean>>({})
  const [planOpen, setPlanOpen] = useState(false)
  const [compatOpen, setCompatOpen] = useState(false)
  const [inPlaceOpen, setInPlaceOpen] = useState(false)
  const [planResult, setPlanResult] = useState<any>(null)
  const [compatResult, setCompatResult] = useState<any>(null)
  const [reportOpen, setReportOpen] = useState(false)
  const [reportLoading, setReportLoading] = useState(false)
  const [reportResult, setReportResult] = useState<any>(null)
  const [submitting, setSubmitting] = useState(false)
  const [activeUpgrade, setActiveUpgrade] = useState<ActiveUpgrade | null>(null)
  const [upgradeStep, setUpgradeStep] = useState(0)
  const [planForm] = Form.useForm()
  const [compatForm] = Form.useForm()
  const [inPlaceForm] = Form.useForm()

  const planInstanceId = Form.useWatch('instance_id', planForm)
  const compatInstanceId = Form.useWatch('instance_id', compatForm)
  const inPlaceInstanceId = Form.useWatch('instance_id', inPlaceForm)
  // executeStrategy is watched inside ExecuteUpgradeModal

  const loadData = () => {
    upgradeApi.listHistory().then((res: any) => setHistory(res?.data || [])).catch(() => setHistory([]))
    instanceApi.list(1000, 0).then((res: any) => setInstances(res?.data || [])).catch(() => setInstances([]))
    setVersionsLoading(true)
    versionApi.listSupported().then((res: any) => setVersions(res?.data || [])).catch(() => setVersions([])).finally(() => setVersionsLoading(false))
  }

  useEffect(() => {
    loadData()
  }, [])

  // Auto-detect version when an instance is selected and version is unknown
  useEffect(() => {
    const ids = [planInstanceId, compatInstanceId, inPlaceInstanceId].filter(Boolean) as string[]
    if (ids.length === 0) return
    ids.forEach((id) => {
      const inst = findInstance(id)
      if (inst && !inst.version?.full_version && !inst.version?.version && !detectingVersions[id]) {
        detectInstanceVersion(id)
      }
    })
  }, [planInstanceId, compatInstanceId, inPlaceInstanceId])

  useEffect(() => {
    if (!history.some((item) => activeUpgradeStatuses.has((item.status || '').toLowerCase()))) return
    const timer = window.setInterval(loadData, 5000)
    return () => window.clearInterval(timer)
  }, [history])

  useEffect(() => {
    if (!activeUpgrade) return
    if (terminalUpgradeStatuses.has((activeUpgrade.status || '').toLowerCase())) return
    const timer = window.setInterval(async () => {
      try {
        const res: any = await upgradeApi.get(activeUpgrade.task_id)
        const data = res?.data
        if (data) {
          setActiveUpgrade((prev) => prev ? {
            ...prev,
            status: data.status || prev.status,
            progress: typeof data.progress === 'number' ? data.progress : prev.progress,
            stage: data.stage || currentUpgradeStage({ ...prev, progress: typeof data.progress === 'number' ? data.progress : prev.progress, status: data.status || prev.status }),
            message: data.message || data.error_message || prev.message,
            task_type: data.task_type || prev.task_type,
            steps: Array.isArray(data.steps) ? data.steps : undefined,
            logs: Array.isArray(data.logs) ? data.logs : prev.logs,
            finished_at: data.finished_at || data.completed_at,
          } : prev)
          const nextStages = upgradeStagesFor({ strategy: activeUpgrade.strategy, task_type: data.task_type || activeUpgrade.task_type })
          setUpgradeStep(inferStepIndex(typeof data.progress === 'number' ? data.progress : activeUpgrade.progress, nextStages, data.status || activeUpgrade.status, data.stage))
          if (terminalUpgradeStatuses.has((data.status || '').toLowerCase())) {
            loadData()
          }
        }
      } catch {}
    }, 3000)
    return () => window.clearInterval(timer)
  }, [activeUpgrade?.task_id, activeUpgrade?.status])

  const instanceOptions = useMemo(
    () => instances.map((i) => ({
      value: i.id,
      label: `${i.name} (${i.connection?.host || i.host || '-'}:${i.connection?.port || i.port || '-'})`,
    })),
    [instances],
  )

  const versionOptions = useMemo(
    () => versions
      .slice()
      .sort((a, b) => {
        if (a.flavor !== b.flavor) return a.flavor.localeCompare(b.flavor)
        return b.release_date.localeCompare(a.release_date)
      })
      .map((v) => ({
        value: v.id,
        label: `${v.flavor} ${v.version}${v.is_lts ? ' [LTS]' : ''}${v.status === 'eol' ? ' [EOL]' : ''}${v.local_available ? ' [存在]' : ' [下载]'}`,
      })),
    [versions],
  )

  const clusterOptions = useMemo(() => {
    const clusterIds = Array.from(new Set(instances.map((i) => i.cluster_id).filter(Boolean)))
    return clusterIds.map((clusterId) => ({
      value: clusterId,
      label: `${clusterId} (${instances.filter((i) => i.cluster_id === clusterId).length} 个实例)`,
    }))
  }, [instances])

  const detectInstanceVersion = async (instanceId: string) => {
    if (!instanceId || detectingVersions[instanceId]) return
    setDetectingVersions((prev) => ({ ...prev, [instanceId]: true }))
    try {
      const res: any = await instanceApi.detectVersion(instanceId)
      const v = res?.data
      if (v?.version || v?.full_version) {
        setInstances((prev) => prev.map((inst) =>
          inst.id === instanceId
            ? { ...inst, version: v as InstanceVersion }
            : inst,
        ))
        message.success(`实例 ${instanceId} 版本检测成功: ${v.full_version || v.version}`)
      } else {
        message.warning(`实例 ${instanceId} 版本检测返回空数据`)
      }
    } catch (err: any) {
      message.warning(`版本检测失败: ${err?.response?.data?.message || err?.message || '未知错误'}`)
    } finally {
      setDetectingVersions((prev) => ({ ...prev, [instanceId]: false }))
    }
  }

  const findInstance = (id?: string) => id ? instances.find((i) => i.id === id) : undefined
  const detectedVersion = (inst?: Instance): string => {
    if (!inst) return '未识别'
    try {
      const raw = inst as Instance & { version_id?: string; target_version_id?: string; full_version?: string; mysql_version?: string; version?: string }
      const pick = (...candidates: (string | number | undefined)[]): string => {
        for (const c of candidates) {
          if (typeof c === 'string' && c) return c
          if (typeof c === 'number') return String(c)
        }
        return ''
      }
      const versionId = pick(inst.connection?.version_id, raw?.version_id, raw?.target_version_id)
      const versionEntry = versionId ? versions.find((v) => v.id === versionId) : undefined
      return pick(
        inst.version?.full_version,
        inst.version?.version,
        raw?.full_version,
        raw?.mysql_version,
        raw?.version,
        versionEntry ? `${versionEntry.flavor} ${versionEntry.version}` : '',
        versionId,
      ) || '未识别'
    } catch {
      return '未识别'
    }
  }

  const versionInfo = (id?: string) => {
    if (!id) return null
    const inst = findInstance(id)
    const version = detectedVersion(inst)
    const isDetecting = detectingVersions[id || '']
    return (
      <Descriptions size="small" bordered column={1} style={{ marginBottom: 16 }}>
        <Descriptions.Item label="当前源版本">
          <Space>
            <Tag color={version === '未识别' ? 'warning' : 'blue'}>{version}</Tag>
            {version === '未识别' && (
              <Button size="small" loading={isDetecting} onClick={() => detectInstanceVersion(id)}>
                {isDetecting ? '检测中...' : '检测版本'}
              </Button>
            )}
            {isDetecting && <Spin size="small" />}
          </Space>
        </Descriptions.Item>
      </Descriptions>
    )
  }

  const planUpgrade = async (values: any) => {
    if (!values.backup_confirmed) {
      message.warning('请先确认数据已完成备份')
      return
    }
    setSubmitting(true)
    try {
      const res: any = await upgradeApi.planPath({
        instance_id: values.instance_id,
        target_version: values.target_version,
        strategy: values.strategy,
      })
      setPlanResult(res?.data)
      message.success('升级路径规划已生成')
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '升级路径规划失败')
    } finally {
      setSubmitting(false)
    }
  }

  const checkCompatibility = async (values: any) => {
    setSubmitting(true)
    try {
      const res: any = await upgradeApi.checkCompat(values)
      setCompatResult(res?.data)
      message.success('兼容性检查完成')
    } catch (err: any) {
      setCompatResult(null)
      message.error(err?.response?.data?.message || err?.message || '兼容性检查失败')
    } finally {
      setSubmitting(false)
    }
  }

  const executeUpgrade = async (values: any) => {
    if (!values.backup_enabled) {
      message.warning('请确认数据已备份后再启动升级')
      return
    }
    setSubmitting(true)
    try {
      const strategy = values.strategy || 'inplace'
      let res: any
      if (strategy === 'logical') {
        res = await upgradeApi.executeLogical({
          instance_id: values.instance_id,
          plan_id: values.plan_id,
          target_version: values.target_version,
          backup_enabled: !!values.backup_enabled,
          parallelism: values.parallelism,
          batch_size: values.batch_size,
        })
      } else if (strategy === 'rolling') {
        res = await upgradeApi.executeRolling({
          cluster_id: values.cluster_id,
          plan_id: values.plan_id,
          target_version: values.target_version,
          max_in_parallel: values.max_in_parallel,
          health_check_interval: values.health_check_interval,
        })
      } else {
        res = await upgradeApi.executeInPlace({
          instance_id: values.instance_id,
          plan_id: values.plan_id,
          target_version: values.target_version,
          backup_enabled: !!values.backup_enabled,
        })
      }
      if (!res?.data?.task_id && !res?.data?.id) {
        throw new Error('upgrade API did not return task_id')
      }
      const taskId = res?.data?.task_id || res?.data?.id
      const activeUpgradeData: ActiveUpgrade = {
        task_id: taskId,
        instance_id: values.instance_id || '',
        cluster_id: values.cluster_id,
        strategy,
        task_type: strategy === 'rolling' ? 'upgrade_rolling' : strategy === 'logical' ? 'upgrade_logical' : 'upgrade_in_place',
        status: res?.data?.status || 'running',
        progress: typeof res?.data?.progress === 'number' ? res.data.progress : 0,
        stage: upgradeStagesFor({ strategy })[0],
        message: '升级任务已提交',
        started_at: new Date().toISOString(),
        steps: [],
        logs: [],
      }
      setActiveUpgrade(activeUpgradeData)
      setUpgradeStep(0)
      message.success('升级任务已提交')
      setInPlaceOpen(false)
      inPlaceForm.resetFields()
      loadData()
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '升级任务提交失败')
    } finally {
      setSubmitting(false)
    }
  }

  const showReport = async (record: UpgradeHistory) => {
    setReportOpen(true)
    setReportLoading(true)
    setReportResult(null)
    try {
      const res: any = await upgradeApi.getReport(record.id)
      setReportResult(res?.data || null)
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '加载升级报告失败')
      setReportOpen(false)
    } finally {
      setReportLoading(false)
    }
  }

  const canRollback = (record: UpgradeHistory) => {
    const type = (record.task_type || record.upgrade_type || '').toLowerCase()
    if (type.includes('rollback')) return false
    const status = (record.status || '').toLowerCase()
    return terminalUpgradeStatuses.has(status) && !!record.instance_id
  }

  const rollbackUpgrade = (record: UpgradeHistory) => {
    Modal.confirm({
      title: '确认回滚升级',
      content: '回滚会停止目标实例、恢复数据和配置，请确认已评估业务影响。',
      okText: '确认回滚',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        setSubmitting(true)
        try {
          const res: any = await upgradeApi.rollback({
            plan_id: record.plan_id || record.id,
            instance_id: record.instance_id,
            force: true,
          })
          const data = res?.data || {}
          message.success(data.rollback_id ? `回滚任务已提交: ${data.rollback_id}` : '回滚任务已提交')
          loadData()
        } catch (err: any) {
          message.error(err?.response?.data?.message || err?.message || '回滚升级失败')
        } finally {
          setSubmitting(false)
        }
      },
    })
  }

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 150, ellipsis: true },
    {
      title: '信息',
      dataIndex: 'message',
      key: 'message',
      width: 260,
      ellipsis: true,
      render: (v: string) => v || '-',
    },
    {
      title: '操作',
      key: 'action',
      fixed: 'right' as const,
      width: 190,
      render: (_: any, record: UpgradeHistory) => (
        <Space size="small">
          <Button size="small" icon={<FileTextOutlined />} onClick={() => showReport(record)}>
            报告
          </Button>
          {canRollback(record) && (
            <Button size="small" danger icon={<RollbackOutlined />} loading={submitting} onClick={() => rollbackUpgrade(record)}>
              {'回滚'}
            </Button>
          )}
        </Space>
      ),
    },
    { title: '实例', dataIndex: 'instance_name', key: 'instance_name', render: (v: string, r: UpgradeHistory) => v || r.instance_id },
    { title: '类型', dataIndex: 'upgrade_type', key: 'upgrade_type', render: (v: string) => <Tag>{v || '-'}</Tag> },
    {
      title: '版本变化',
      key: 'version',
      render: (_: any, r: UpgradeHistory) => `${r.source_version || '-'} -> ${r.target_version || '-'}`,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (s: string) => <Tag color={isCompletedUpgradeStatus(s) ? 'success' : isFailedUpgradeStatus(s) ? 'error' : terminalUpgradeStatuses.has((s || '').toLowerCase()) ? 'default' : 'processing'}>{s || '-'}</Tag>,
    },
    {
      title: '进度',
      dataIndex: 'progress',
      key: 'progress',
      width: 160,
      render: (p: number) => <Progress percent={p || 0} size="small" />,
    },
    { title: '阶段', dataIndex: 'stage', key: 'stage', render: (v: string) => v || '-' },
    { title: '时间', dataIndex: 'start_time', key: 'start_time', render: (v: string, r: UpgradeHistory) => v || r.created_at || '-' },
  ]

  // activeUpgradeStages, activeUpgradeSteps, activeUpgradeCurrentStage, activeUpgradeSubsteps moved to UpgradeProgressCard

  return (
    <div>
      <Title level={4}>版本升级管理</Title>

      <Card style={{ marginBottom: 16 }}>
        <Space wrap>
          <Button type="primary" icon={<FileTextOutlined />} onClick={() => setPlanOpen(true)}>规划升级路径</Button>
          <Button icon={<CheckCircleOutlined />} onClick={() => setCompatOpen(true)}>兼容性检查</Button>
          <Button danger icon={<PlayCircleOutlined />} onClick={() => setInPlaceOpen(true)}>启动升级任务</Button>
          <Button icon={<ReloadOutlined />} onClick={loadData}>刷新</Button>
        </Space>
      </Card>

      <Table columns={columns} dataSource={history} rowKey="id" scroll={{ x: 1000 }} />

      {activeUpgrade && (
        <UpgradeProgressCard
          activeUpgrade={activeUpgrade}
          upgradeStep={upgradeStep}
          planResult={planResult}
        />
      )}

      <UpgradePlanModal
        open={planOpen}
        submitting={submitting}
        versionsLoading={versionsLoading}
        instanceOptions={instanceOptions}
        versionOptions={versionOptions}
        planResult={planResult}
        versionInfo={versionInfo(planInstanceId)}
        form={planForm}
        onCancel={() => setPlanOpen(false)}
        onFinish={planUpgrade}
      />

      <CompatCheckModal
        open={compatOpen}
        submitting={submitting}
        instanceOptions={instanceOptions}
        versionOptions={versionOptions}
        compatResult={compatResult}
        versionInfo={versionInfo(compatInstanceId)}
        form={compatForm}
        onCancel={() => { setCompatOpen(false); setCompatResult(null) }}
        onFinish={checkCompatibility}
      />

      <ExecuteUpgradeModal
        open={inPlaceOpen}
        submitting={submitting}
        instanceOptions={instanceOptions}
        clusterOptions={clusterOptions}
        versionOptions={versionOptions}
        versionsLoading={versionsLoading}
        versionInfo={versionInfo(inPlaceInstanceId)}
        form={inPlaceForm}
        onCancel={() => setInPlaceOpen(false)}
        onFinish={executeUpgrade}
      />
      <UpgradeReportModal
        open={reportOpen}
        loading={reportLoading}
        result={reportResult}
        onClose={() => setReportOpen(false)}
      />
    </div>
  )
}

export default UpgradeManage
