import React, { useEffect, useRef, useState } from 'react'
import {
  Button, Card, Checkbox, Col, Form, Input, InputNumber, message, Modal, Row, Select, Space, Tabs, Typography,
} from 'antd'
import { ClusterOutlined, EyeOutlined, KeyOutlined, PlayCircleOutlined, ReloadOutlined } from '@ant-design/icons'
import { clusterDeployApi, hostApi, instanceApi, versionApi, type Host, type Instance, type VersionEntry } from '../services/api'
import { getDefaultMySQLCredential, setDefaultMySQLCredential } from '../services/sessionSecrets'
import { formatClusterRole } from '../services/roleDisplay'
import { useTaskSSE, type TaskEvent } from '../services/useTaskSSE'
import { processStepEvent } from '../services/deployStepHelper'
import {
  ArchType, DeployResult,
  getStatusCategory,
  isCompletedDeployStatus, isFailedDeployStatus, isPartialDeployStatus, isTerminalDeployStatus,
  deploymentProgress,
  versionSupportsArch,
  createMgrGroupName, normalizeDeployment, buildDeployPayload,
  STAGE_ORDER,
} from '../services/deployHelpers'
import PlanPreviewModal from '../components/PlanPreviewModal'
import DeployCredentialsModal from '../components/DeployCredentialsModal'
import DeployErrorModal from '../components/DeployErrorModal'
import MySQLPasswordModal from '../components/MySQLPasswordModal'
import ClusterDeployProgress from '../components/ClusterDeployProgress'
import DeploymentHistoryPanel from '../components/DeploymentHistoryPanel'
import PrecheckResultTable from '../components/PrecheckResultTable'

const { Text } = Typography

const ClusterDeploy: React.FC = () => {
  const [hosts, setHosts] = useState<Host[]>([])
  const [instances, setInstances] = useState<Instance[]>([])
  const [versions, setVersions] = useState<VersionEntry[]>([])
  const [tab, setTab] = useState<ArchType>('ha')
  const [submitting, setSubmitting] = useState(false)
  const [historyLoading, setHistoryLoading] = useState(false)
  const [deployments, setDeployments] = useState<DeployResult[]>([])
  const [statusFilter, setStatusFilter] = useState<string[]>(['success', 'running'])
  const [archFilter, setArchFilter] = useState<ArchType | 'all'>('all')
  const [showHistory, setShowHistory] = useState(true)
  const [precheckResults, setPrecheckResults] = useState<any[] | null>(null)
  const [precheckLoading, setPrecheckLoading] = useState(false)
  const [precheckContext, setPrecheckContext] = useState<{ arch: ArchType; values: any } | null>(null)
  const [activeDeployment, setActiveDeployment] = useState<DeployResult | null>(null)
  const [currentStep, setCurrentStep] = useState(0)
  const [credentialModalResult, setCredentialModalResult] = useState<{
    visible: boolean
    mysql_user: string
    mysql_password: string
    nodes?: Array<{ host: string; port?: number; role?: string; username?: string; password?: string }>
  }>({ visible: false, mysql_user: '', mysql_password: '' })
  const [deployErrorDetail, setDeployErrorDetail] = useState<DeployResult | null>(null)
  const [credential, setCredential] = useState<{ username: string; password: string }>(() => getDefaultMySQLCredential())
  const [mysqlPasswordModalOpen, setMysqlPasswordModalOpen] = useState(false)
  const [mysqlPasswordForm] = Form.useForm()

  const handleSaveMysqlPassword = async () => {
    try {
      const values = await mysqlPasswordForm.validateFields()
      const newCredential = { username: values.username || 'root', password: values.password }
      setCredential(newCredential)
      setDefaultMySQLCredential(newCredential)
      setMysqlPasswordModalOpen(false)
      message.success('MySQL root 密码已保存')
    } catch {
      // validation failed
    }
  }
  const pollRef = useRef<number | null>(null)
  const historyPollRef = useRef<number | null>(null)

  // Plan preview state
  const [planPreviewOpen, setPlanPreviewOpen] = useState(false)
  const [planPreviewData, setPlanPreviewData] = useState<any>(null)
  const [planPreviewArch, setPlanPreviewArch] = useState<ArchType>('ha')
  const [planPreviewLoading, setPlanPreviewLoading] = useState(false)

  const [haForm] = Form.useForm()
  const [mhaForm] = Form.useForm()
  const [mgrForm] = Form.useForm()
  const [pxcForm] = Form.useForm()

  useEffect(() => {
    hostApi.list(100, 0).then((res: any) => setHosts(res?.data || [])).catch(() => { /* BUG-014: host list load failure is non-critical, page works with empty list */ })
    instanceApi.list(1000, 0).then((res: any) => setInstances(res?.data || [])).catch(() => { /* BUG-014: instance list load failure is non-critical */ })
    versionApi.listSupported().then((res: any) => setVersions(res?.data || [])).catch(() => { /* BUG-014: version list load failure is non-critical */ })
    loadDeployments()
  }, [])

  useEffect(() => () => {
    if (pollRef.current) window.clearInterval(pollRef.current)
    if (historyPollRef.current) window.clearInterval(historyPollRef.current)
  }, [])

  useEffect(() => {
    if (mgrForm.getFieldValue('group_name')) return
    mgrForm.setFieldValue('group_name', createMgrGroupName())
  }, [mgrForm])

  useEffect(() => {
    if (historyPollRef.current) {
      window.clearInterval(historyPollRef.current)
      historyPollRef.current = null
    }
    const hasRunningDeployment = deployments.some((item) => !isTerminalDeployStatus(item.status))
    if (!showHistory || !hasRunningDeployment) return
    historyPollRef.current = window.setInterval(() => {
      loadDeployments(false)
    }, 5000)
    return () => {
      if (historyPollRef.current) {
        window.clearInterval(historyPollRef.current)
        historyPollRef.current = null
      }
    }
  }, [deployments, showHistory])

  const hostOptions = hosts.map((h) => ({ value: h.id, label: `${h.name} (${h.address})` }))

  const loadDeployments = async (showLoading = true) => {
    if (showLoading) setHistoryLoading(true)
    try {
      const res: any = await clusterDeployApi.list(1000, 0)
      const allData = (Array.isArray(res?.data) ? res.data : []).map(normalizeDeployment)
      setDeployments(allData)
    } catch {
      setDeployments([])
    } finally {
      if (showLoading) setHistoryLoading(false)
    }
  }

  const showDeploymentResultMessage = (dep: DeployResult) => {
    const arch = dep.cluster_type?.toUpperCase?.() || 'Cluster'
    if (isCompletedDeployStatus(dep.status)) {
      message.success(`${arch} 集群部署完成`)
      // Show credentials modal on successful deployment
      if (dep.mysql_user || dep.mysql_password) {
        setCredentialModalResult({
          visible: true,
          mysql_user: dep.mysql_user || '-',
          mysql_password: dep.mysql_password || '-',
          nodes: (dep.nodes || []).map((node) => ({
            host: node.host || '-',
            port: node.port,
            role: formatClusterRole(dep.cluster_type, node.role),
            username: dep.mysql_user || 'root',
            password: dep.mysql_password || '-',
          })),
        })
      }
    } else if (isPartialDeployStatus(dep.status)) message.warning(`${arch} 集群部署部分完成: ${dep.message || dep.status}`)
    else if (isFailedDeployStatus(dep.status)) {
      setDeployErrorDetail(dep)
      message.error(`${arch} 集群部署失败: ${dep.message || dep.status}`)
    }
  }

  const stopPolling = () => {
    if (pollRef.current) {
      window.clearInterval(pollRef.current)
      pollRef.current = null
    }
  }

  const patchDeployment = (dep: DeployResult) => {
    const fresh = { ...dep, _ts: Date.now() }
    setDeployments((items) => {
      const exists = items.some((item) => item.deployment_id === dep.deployment_id)
      if (!exists) return [fresh, ...items]
      return items.map((item) => (item.deployment_id === dep.deployment_id ? fresh : item))
    })
    setActiveDeployment((cur) => (cur && cur.deployment_id === dep.deployment_id ? fresh : cur))
  }

  const patchActiveDeploymentFromSSE = (event: TaskEvent) => {
    setActiveDeployment((current) => {
      if (!current || current.deployment_id !== event.task_id) return current
      const next: DeployResult = {
        ...current,
        status: event.status || current.status,
        stage: event.stage || current.stage,
        progress: typeof event.progress === 'number' ? event.progress : current.progress,
        message: event.log_line || current.message,
        logs: event.log_line ? [...(current.logs || []), event.log_line] : current.logs,
      }
      setDeployments((items) => items.map((item) => (item.deployment_id === next.deployment_id ? { ...next, _ts: Date.now() } : item)))
      return next
    })
  }

  useTaskSSE({
    taskID: activeDeployment?.deployment_id || '',
    enabled: !!activeDeployment && !isTerminalDeployStatus(activeDeployment.status),
    onProgress: patchActiveDeploymentFromSSE,
    onLog: patchActiveDeploymentFromSSE,
    onStep: (event) => {
      const stepName = event.metadata?.step_name
      if (!stepName) return
      setActiveDeployment((current) =>
        processStepEvent(current as any, event.task_id, stepName, event.metadata?.step_status, event.metadata?.step_message) as DeployResult | null
      )
    },
    onStatus: (event) => {
      patchActiveDeploymentFromSSE(event)
      if (event.status && ['completed', 'success', 'failed', 'error', 'partial', 'partial_success', 'destroyed'].includes(event.status.toLowerCase())) {
        loadDeployments(false)
      }
    },
  })

  const startPolling = (dep: DeployResult) => {
    if (isTerminalDeployStatus(dep.status)) return
    stopPolling()
    let attempts = 0
    pollRef.current = window.setInterval(async () => {
      attempts += 1
      try {
        const res: any = await clusterDeployApi.getStatus(dep.deployment_id)
        const data = res?.data
        if (!data) return
        const next: DeployResult = {
          ...dep,
          status: data.status || dep.status,
          stage: data.stage,
          progress: typeof data.progress === 'number' ? data.progress : dep.progress,
          message: data.message || dep.message,
          finished_at: data.finished_at,
          nodes: Array.isArray(data.nodes) ? data.nodes : dep.nodes,
          steps: Array.isArray(data.steps) ? data.steps : dep.steps,
          logs: Array.isArray(data.logs) ? data.logs : dep.logs,
        }
        patchDeployment(next)
        if (isTerminalDeployStatus(next.status)) loadDeployments(false)
        const stepIdx = next.stage ? STAGE_ORDER.indexOf(next.stage as typeof STAGE_ORDER[number]) : -1
        if (stepIdx >= 0) setCurrentStep(stepIdx)
        if (isTerminalDeployStatus(next.status) || attempts > 600) {
          stopPolling()
          showDeploymentResultMessage(next)
        }
      } catch {
        // Polling is retried until deployment reaches a terminal state.
      }
    }, 2000)
  }

  const cred = { username: credential.username, password: credential.password }

  const doPreview = (arch: ArchType, values: any) => {
    if (!credential.password) {
      message.error('请先设置MySQL root密码（点击"MySQL密码"按钮）')
      return
    }
    if (!versionSupportsArch(arch, values.mysql_version)) {
      message.error('MGR 部署需要选择 MySQL 8.0+ 版本，MySQL 5.7 不支持 Group Replication')
      return
    }
    const nextValues = { ...values }
    if (arch === 'mgr' && !nextValues.group_name) {
      nextValues.group_name = createMgrGroupName()
      mgrForm.setFieldValue('group_name', nextValues.group_name)
    }
    const payload = buildDeployPayload(arch, nextValues, cred)
    setPlanPreviewLoading(true)
    setPlanPreviewArch(arch)
    clusterDeployApi.validateCluster(payload).then((res: any) => {
      const plan = res?.data?.plan || res?.data
      setPlanPreviewData(plan)
      setPlanPreviewOpen(true)
    }).catch((err: any) => {
      message.error(`计划验证失败: ${err?.response?.data?.message || err?.message}`)
    }).finally(() => {
      setPlanPreviewLoading(false)
    })
  }

  const runDeploy = (arch: ArchType, values: any) => {
    doDeploy(arch, values)
  }

  const runPrecheck = async (arch: ArchType, values: any) => {
    if (!versionSupportsArch(arch, values.mysql_version)) {
      message.error('MGR 部署需要选择 MySQL 8.0+ 版本，MySQL 5.7 不支持 Group Replication')
      return
    }
    const nextValues = { ...values }
    if (arch === 'mgr' && !nextValues.group_name) {
      nextValues.group_name = createMgrGroupName()
      mgrForm.setFieldValue('group_name', nextValues.group_name)
    }
    const payload = buildDeployPayload(arch, nextValues, cred)
    const nodes = payload.nodes || []
    const hostIDs: string[] = nodes.map((node: any) => node.host_id).filter(Boolean)
    if (hostIDs.length === 0) {
      message.warning('请先选择部署节点主机')
      return
    }
    setPrecheckLoading(true)
    setPrecheckResults(null)
    try {
      setPrecheckContext({ arch, values: nextValues })
      const res: any = await clusterDeployApi.precheck({ cluster_type: arch, host_ids: hostIDs, nodes })
      setPrecheckResults(res?.data || [])
      const failed = (res?.data || []).filter((r: any) => r.status === 'fail')
      if (failed.length > 0) {
        message.error(`${failed.length} 台主机环境检查不通过，请先修复后再部署`)
      } else {
        message.success('所有节点环境检查通过')
      }
    } catch (err: any) {
      message.error(`环境预检失败: ${err?.response?.data?.message || err?.message}`)
    } finally {
      setPrecheckLoading(false)
    }
  }

  const viewDeployPlan = async (record: DeployResult) => {
    try {
      const res: any = await clusterDeployApi.getDeployPlan(record.deployment_id)
      const plan = res?.data || res
      setPlanPreviewArch(record.cluster_type)
      setPlanPreviewData(plan)
      setPlanPreviewOpen(true)
    } catch (err: any) {
      message.error(`获取部署计划失败: ${err?.response?.data?.message || err?.message}`)
    }
  }

  const viewDeploymentDetail = async (record: DeployResult) => {
    try {
      const res: any = await clusterDeployApi.getStatus(record.deployment_id)
      const dep = normalizeDeployment(res?.data || record)
      setActiveDeployment(dep)
      const stepIdx = dep.stage ? STAGE_ORDER.indexOf(dep.stage as typeof STAGE_ORDER[number]) : -1
      if (stepIdx >= 0) setCurrentStep(stepIdx)
      if (!isTerminalDeployStatus(dep.status)) startPolling(dep)
    } catch (err: any) {
      message.error(`获取部署详情失败: ${err?.response?.data?.message || err?.message}`)
    }
  }

  const doDeploy = async (arch: ArchType, values: any, payloadOverride?: any) => {
    setSubmitting(true)
    setCurrentStep(0)
    setActiveDeployment(null)
    const payload = payloadOverride || buildDeployPayload(arch, values, cred)
    try {
      await clusterDeployApi.validateCluster(payload)
      const res: any = await clusterDeployApi.deployCluster(payload)
      if (!res?.data?.deployment_id && !res?.data?.id) {
        throw new Error('backend did not return deployment_id')
      }
      const data = res?.data
      const status = data?.status || 'running'
      const dep: DeployResult = {
        deployment_id: data?.deployment_id || data?.id,
        cluster_id: data?.cluster_id || values.cluster_id || data?.deployment_id || data?.id,
        cluster_type: data?.cluster_type || arch,
        status,
        progress: deploymentProgress(status, data?.progress),
        stage: isCompletedDeployStatus(status) ? STAGE_ORDER[4] : STAGE_ORDER[0],
        message: data?.message || '部署已提交，等待后端开始执行',
        mysql_user: data?.mysql_user,
        mysql_password: data?.mysql_password,
        started_at: data?.started_at || new Date().toISOString(),
        finished_at: data?.finished_at,
        nodes: Array.isArray(data?.nodes) ? data.nodes : [],
        steps: Array.isArray(data?.steps) ? data.steps : [],
        logs: Array.isArray(data?.logs) ? data.logs : [],
      }
      setActiveDeployment(dep)
      await loadDeployments()
      if (isTerminalDeployStatus(dep.status)) {
        showDeploymentResultMessage(dep)
      } else {
        message.success(`${arch.toUpperCase()} 集群部署任务已提交`)
      }
      startPolling(dep)
    } catch (err: any) {
      const failedData = err?.response?.data?.data
      if (failedData?.deployment_id || failedData?.id) {
        const dep = normalizeDeployment({
          ...failedData,
          cluster_id: failedData.cluster_id || values.cluster_id || payload.cluster_id,
          cluster_type: failedData.cluster_type || arch,
          message: failedData.error_message || failedData.message || err?.message,
        })
        patchDeployment(dep)
        const failedStepIdx = dep.steps?.findIndex((step) => isFailedDeployStatus(step.status)) ?? -1
        if (failedStepIdx >= 0) setCurrentStep(Math.min(failedStepIdx, STAGE_ORDER.length - 1))
        await loadDeployments(false)
        showDeploymentResultMessage(dep)
        return
      }
      message.error(`提交部署失败: ${err?.response?.data?.message || err?.message}`)
    } finally {
      setSubmitting(false)
    }
  }

  const repairPrecheckItem = async (record: any, detail: any) => {
    if (!record?.host_id || !detail?.port) {
      message.error('缺少修复所需的主机或端口信息')
      return
    }
    setPrecheckLoading(true)
    try {
      await clusterDeployApi.repairPrecheck({
        host_id: record.host_id,
        port: detail.port,
        action: detail.fix_action || 'repair_component_config',
        component: detail.payload?.component,
        basedir: detail.payload?.basedir,
        data_dir: detail.payload?.data_dir,
        package_url: detail.payload?.package_url,
        relay_url: detail.payload?.relay_url,
      })
      message.success('修复完成，正在重新执行环境预检')
      if (precheckContext) {
        await runPrecheck(precheckContext.arch, precheckContext.values)
      }
    } catch (err: any) {
      message.error(`修复失败: ${err?.response?.data?.message || err?.message}`)
    } finally {
      setPrecheckLoading(false)
    }
  }

  const destroyDeployment = (record: DeployResult) => {
    Modal.confirm({
      title: `销毁集群 ${record.deployment_id}?`,
      content: (
        <div>
          <p><strong>销毁流程：</strong></p>
          <ol style={{ paddingLeft: 20 }}>
            <li>对所有实例执行完整备份</li>
            <li>验证备份文件（路径、大小、校验和）</li>
            <li>清理远程主机上的数据库目录和服务</li>
            <li>删除平台纳管关系和拓扑</li>
          </ol>
          <p style={{ color: '#ff4d4f', marginTop: 8 }}>⚠️ 此操作会删除数据库服务和数据目录，无法撤销</p>
          <p style={{ color: '#ff4d4f' }}>⚠️ 如果备份失败，销毁操作将被拒绝</p>
        </div>
      ),
      okText: '确认销毁',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: async () => {
        try {
          const res: any = await clusterDeployApi.destroy(record.deployment_id)
          const next: DeployResult = {
            ...record,
            status: 'destroyed',
            progress: 100,
            message: res?.data?.message || '集群已销毁',
            finished_at: new Date().toISOString(),
          }
          patchDeployment(next)
          await loadDeployments()
          message.success('集群销毁成功：已完成备份验证和远程清理')
        } catch (err: any) {
          await loadDeployments()
          message.error(`销毁失败: ${err?.response?.data?.message || err?.message || '未知错误'}`)
        }
      },
    })
  }

  const renderForm = (
    arch: ArchType,
    form: any,
    extraFields: React.ReactNode,
    onFinish: (values: any) => void,
    options?: { simpleReplica?: boolean },
  ) => (
    <Form form={form} layout="horizontal" onFinish={onFinish}>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="mysql_version" label="MySQL版本" initialValue="8.0" rules={[{ required: true }]} labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
            <Select
              placeholder="选择版本"
              showSearch
              optionFilterProp="label"
              options={versions
                .slice()
                .filter((v) => versionSupportsArch(arch, v.version))
                .sort((a, b) => {
                  if (a.flavor !== b.flavor) return a.flavor.localeCompare(b.flavor)
                  return b.release_date.localeCompare(a.release_date)
                })
                .map((v) => ({
                  value: v.version,
                  label: `${v.flavor} ${v.version}${v.is_lts ? ' [LTS]' : ''}${v.local_available ? ' [存在]' : ' [下载]'}${v.min_glibc ? ` (glibc>=${v.min_glibc})` : ''}`,
                }))}
            />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="mysql_port" label="主节点端口" initialValue={3306} rules={[{ required: true, message: '请输入主节点端口' }]} labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
            <InputNumber min={1} max={65535} style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="cluster_id" label="集群ID" rules={[{ required: true, message: '请输入集群ID' }]} labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
            <Input placeholder={`${arch}-cluster-01`} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="master_host_id" label="主节点" rules={[{ required: true }]} labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
            <Select options={hostOptions} placeholder="选择主节点主机" />
          </Form.Item>
        </Col>
      </Row>
      {!options?.simpleReplica && (
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="replica_host_ids" label="从节点" rules={[{ required: true }]} labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
              <Select mode="multiple" options={hostOptions} placeholder="至少选择1个从节点" maxTagCount={2} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="replica_port" label="从节点端口" labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
              <InputNumber min={1} max={65535} placeholder="默认同主节点端口" style={{ width: '100%' }} />
            </Form.Item>
          </Col>
        </Row>
      )}
      {options?.simpleReplica && (
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="replica_host_id" label="从节点" rules={[{ required: true }]} labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
              <Select options={hostOptions} placeholder="选择从节点主机" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="replica_port" label="从节点端口" initialValue={3310} labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
              <InputNumber min={1} max={65535} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
        </Row>
      )}
      {extraFields && <Row gutter={16}>{extraFields}</Row>}
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="repl_user" label="复制用户" initialValue="repl" labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
            <Input placeholder="repl" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="repl_password" label="复制密码" initialValue="Repl#2024" labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
            <Input.Password placeholder="Repl#2024" />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item label="中间件插件" tooltip="部署完成后自动装配所选中间件" labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
            <Space>
              <Form.Item name="enable_keepalived" valuePropName="checked" noStyle>
                <Checkbox>Keepalived</Checkbox>
              </Form.Item>
              <Form.Item name="enable_proxysql" valuePropName="checked" noStyle>
                <Checkbox>ProxySQL</Checkbox>
              </Form.Item>
            </Space>
          </Form.Item>
        </Col>
        <Col span={12} />
      </Row>
      <Form.Item wrapperCol={{ offset: 0 }}>
        <Space>
          <Button type="primary" icon={<PlayCircleOutlined />} htmlType="submit" loading={submitting}>
            启动部署
          </Button>
          <Button icon={<EyeOutlined />} loading={planPreviewLoading} onClick={() => form.validateFields().then((values: any) => doPreview(arch, values)).catch(() => { /* validation error shown by antd */ })}>
            预览计划
          </Button>
          <Button icon={<ReloadOutlined />} loading={precheckLoading} onClick={() => form.validateFields().then((values: any) => runPrecheck(arch, values)).catch(() => { /* validation error shown by antd */ })}>
            环境预检
          </Button>
        </Space>
      </Form.Item>
      {precheckResults && (
        <PrecheckResultTable
          results={precheckResults}
          loading={precheckLoading}
          onRepair={repairPrecheckItem}
        />
      )}
    </Form>
  )

  const filteredDeployments = deployments.filter((d) => {
    const statusMatch = statusFilter.length === 0 || statusFilter.includes(getStatusCategory(d.status))
    const archMatch = archFilter === 'all' || d.cluster_type === archFilter
    return statusMatch && archMatch
  })

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title={
          <Space>
            <ClusterOutlined />
            <span>集群部署</span>
          </Space>
        }
        extra={
          <Space>
            <Button
              icon={<KeyOutlined />}
              onClick={() => {
                mysqlPasswordForm.setFieldsValue({ username: credential.username || 'root', password: credential.password || 'Root#1234' })
                setMysqlPasswordModalOpen(true)
              }}
            >
              MySQL密码 {credential.password ? '(已设置)' : '(未设置)'}
            </Button>
            <Button type="primary" icon={<ClusterOutlined />} onClick={() => setShowHistory(!showHistory)}>
              {showHistory ? '返回部署' : '部署历史'}
            </Button>
          </Space>
        }
      >
        {showHistory ? (
          <DeploymentHistoryPanel
            loading={historyLoading}
            dataSource={filteredDeployments}
            statusFilter={statusFilter}
            archFilter={archFilter}
            instances={instances}
            onStatusFilterChange={setStatusFilter}
            onArchFilterChange={setArchFilter}
            onViewDetail={viewDeploymentDetail}
            onViewPlan={viewDeployPlan}
            onDestroy={destroyDeployment}
          />
        ) : (
          <Tabs
            activeKey={tab}
            onChange={(key) => setTab(key as ArchType)}
            items={[
              {
                key: 'ha',
                label: 'HA 主从',
                children: renderForm('ha', haForm,
                  null,
                  (values) => runDeploy('ha', values),
                ),
              },
              {
                key: 'mha',
                label: 'MHA 部署',
                children: renderForm('mha', mhaForm,
                  <>
                    <Col span={12}>
                      <Form.Item name="manager_host_id" label="Manager主机" rules={[{ required: true }]} labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
                        <Select options={hostOptions} placeholder="选择 Manager 主机" />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="manager_port" label="Manager端口" initialValue={3306} labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
                        <InputNumber min={1} max={65535} placeholder="默认同主节点端口" style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="vip" label="虚拟IP (VIP)" labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
                        <Input placeholder="如 192.168.1.100" />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="vip_interface" label="VIP网口" labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
                        <Input placeholder="如 eth0" />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="ssh_user" label="SSH用户" initialValue="root" labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
                        <Input placeholder="root" />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="ping_interval" label="健康检查间隔(s)" initialValue="3" labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
                        <InputNumber min={1} max={60} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="ping_retry" label="重试次数" initialValue="5" labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
                        <InputNumber min={1} max={30} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                  </>,
                  (values) => runDeploy('mha', values),
                ),
              },
              {
                key: 'mgr',
                label: 'MGR 部署',
                children: renderForm('mgr', mgrForm,
                  <>
                    <Col span={12}>
                      <Form.Item name="group_name" label="MGR组名" labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
                        <Input />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="local_port" label="组通信端口" initialValue={33061} labelCol={{ span: 8 }} wrapperCol={{ span: 16 }}>
                        <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                  </>,
                  (values) => runDeploy('mgr', values),
                ),
              },
              {
                key: 'pxc',
                label: 'PXC 部署',
                children: renderForm('pxc', pxcForm,
                  <>
                    <Col span={8}>
                      <Form.Item name="wsrep_port" label="wsrep端口" initialValue={4567} labelCol={{ span: 10 }} wrapperCol={{ span: 14 }}>
                        <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item name="cluster_name" label="集群名称" labelCol={{ span: 10 }} wrapperCol={{ span: 14 }}>
                        <Input placeholder="默认使用集群ID" />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item name="sst_method" label="SST方式" initialValue="xtrabackup-v2" labelCol={{ span: 10 }} wrapperCol={{ span: 14 }}>
                        <Select options={[{ value: 'xtrabackup-v2', label: 'xtrabackup-v2' }, { value: 'mariabackup', label: 'mariabackup' }]} />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item name="wsrep_sst_port" label="SST端口" labelCol={{ span: 10 }} wrapperCol={{ span: 14 }}>
                        <InputNumber min={1} max={65535} placeholder="默认4444" style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item name="wsrep_ssl_enabled" label="wsrep SSL" valuePropName="checked" initialValue={false} labelCol={{ span: 10 }} wrapperCol={{ span: 14 }}>
                        <Checkbox>启用</Checkbox>
                      </Form.Item>
                    </Col>
                  </>,
                  (values) => runDeploy('pxc', values),
                ),
              },
            ]}
          />
        )}
      </Card>

      <PlanPreviewModal
        open={planPreviewOpen}
        arch={planPreviewArch}
        data={planPreviewData}
        onClose={() => {
          setPlanPreviewOpen(false)
          setPlanPreviewData(null)
        }}
      />

      <DeployCredentialsModal
        visible={credentialModalResult.visible}
        mysql_user={credentialModalResult.mysql_user}
        mysql_password={credentialModalResult.mysql_password}
        nodes={credentialModalResult.nodes}
        arch={planPreviewArch}
        onClose={() => setCredentialModalResult({ visible: false, mysql_user: '', mysql_password: '' })}
      />

      <DeployErrorModal
        open={!!deployErrorDetail}
        detail={deployErrorDetail}
        onClose={() => setDeployErrorDetail(null)}
      />

      <MySQLPasswordModal
        open={mysqlPasswordModalOpen}
        form={mysqlPasswordForm}
        onClose={() => setMysqlPasswordModalOpen(false)}
        onSave={handleSaveMysqlPassword}
      />

      {activeDeployment && (
        <ClusterDeployProgress
          activeDeployment={activeDeployment}
          currentStep={currentStep}
          onReturn={() => {
            setActiveDeployment(null)
            stopPolling()
          }}
        />
      )}
    </div>
  )
}

export default ClusterDeploy
