import React, { useEffect, useRef, useState } from 'react'
import {
  Alert, Button, Card, Checkbox, Col, Descriptions, Empty, Form, Input, InputNumber, message, Modal, Progress, Row, Select, Space, Steps, Table, Tabs, Tag, Typography,
} from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, ClusterOutlined, DeleteOutlined, EyeOutlined, KeyOutlined, PlayCircleOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { clusterDeployApi, hostApi, instanceApi, versionApi, type Host, type Instance, type VersionEntry } from '../services/api'
import { getDefaultMySQLCredential, setDefaultMySQLCredential } from '../services/sessionSecrets'
import { formatClusterRole } from '../services/roleDisplay'

const { Text } = Typography

type ArchType = 'ha' | 'mha' | 'mgr' | 'pxc'

const createMgrGroupName = () => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  const hex = `${Date.now().toString(16)}${Math.random().toString(16).slice(2)}00000000000000000000000000000000`.slice(0, 32)
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-4${hex.slice(13, 16)}-8${hex.slice(17, 20)}-${hex.slice(20, 32)}`
}

const STAGE_ORDER = ['环境检查', '安装二进制', '配置集群', '启动节点', '集群验证']
const STEP_TYPE_CN: Record<string, string> = { validate: '校验', sync: '同步', bootstrap: '引导', join: '加入', deploy: '部署', configure: '配置', verify: '验证' }

const DEPLOY_SUBSTEPS: Record<string, string[]> = {
  '环境检查': ['检查主机连通性', '验证端口可用性', '检查磁盘空间', '检查系统依赖'],
  '安装二进制': ['下载 MySQL 安装包', '解压安装包', '创建数据目录', '设置文件权限'],
  '配置集群': ['生成 my.cnf', '配置复制用户', '配置复制参数', '写入集群拓扑'],
  '启动节点': ['启动 MySQL 服务', '等待端口就绪', '执行健康检查', '验证服务状态'],
  '集群验证': ['检查复制延迟', '验证 GTID 一致性', '执行数据校验', '生成部署报告'],
}

interface DeployNodeProgress {
  instance_id?: string
  name?: string
  host?: string
  port?: number
  role?: string
  status?: string
  current_step?: string
  progress?: number
  message?: string
}

interface DeployResult {
  deployment_id: string
  cluster_id: string
  cluster_type: ArchType
  status: 'pending' | 'running' | 'success' | 'completed' | 'succeeded' | 'ok' | 'failed' | 'error' | 'timeout' | 'cancelled' | 'canceled' | 'partial' | 'partial_success' | 'destroyed' | string
  stage?: string
  progress: number
  message: string
  mysql_user?: string
  mysql_password?: string
  started_at: string
  finished_at?: string
  nodes?: DeployNodeProgress[]
  steps?: Array<{ name: string; status: string; message?: string; started_at?: string; completed_at?: string }>
  logs?: string[]
}

type DeployStepView = NonNullable<DeployResult['steps']>[number] & {
  id?: string
  type?: string
  target_node?: string
  depends_on?: string[]
}

const normalizeStatus = (status?: string) => (status || '').trim().toLowerCase()

const getStatusCategory = (status?: string) => {
  const norm = normalizeStatus(status)
  if (['success', 'completed', 'succeeded', 'ok'].includes(norm)) return 'success'
  if (['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(norm)) return 'failed'
  if (['partial', 'partial_success'].includes(norm)) return 'partial'
  if (norm === 'destroyed') return 'destroyed'
  if (norm === 'running') return 'running'
  if (norm === 'pending') return 'pending'
  return norm || 'unknown'
}

const isCompletedDeployStatus = (status?: string) => getStatusCategory(status) === 'success'

const isFailedDeployStatus = (status?: string) => getStatusCategory(status) === 'failed'

const isPartialDeployStatus = (status?: string) => getStatusCategory(status) === 'partial'

const isDestroyedDeployStatus = (status?: string) => getStatusCategory(status) === 'destroyed'

const isTerminalDeployStatus = (status?: string) =>
  isCompletedDeployStatus(status) || isFailedDeployStatus(status) || isPartialDeployStatus(status) || isDestroyedDeployStatus(status)

const deploymentProgress = (status?: string, progress?: number) => {
  if (typeof progress === 'number') return progress
  return isTerminalDeployStatus(status) ? 100 : 0
}

const deploymentProgressStatus = (status?: string) => {
  if (isFailedDeployStatus(status)) return 'exception'
  if (isCompletedDeployStatus(status) || isDestroyedDeployStatus(status)) return 'success'
  if (isPartialDeployStatus(status)) return 'normal'
  return 'active'
}

const deploymentStepStatus = (status?: string) => {
  if (isFailedDeployStatus(status)) return 'error'
  if (isCompletedDeployStatus(status) || isDestroyedDeployStatus(status)) return 'finish'
  if (isPartialDeployStatus(status)) return 'error'
  return 'process'
}

const clampProgress = (progress?: number) => {
  if (typeof progress !== 'number' || Number.isNaN(progress)) return 0
  return Math.max(0, Math.min(100, Math.round(progress)))
}

const stepStatusToAntd = (status?: string): 'wait' | 'process' | 'finish' | 'error' => {
  const norm = normalizeStatus(status)
  if (['completed', 'success', 'succeeded', 'ok', 'done'].includes(norm)) return 'finish'
  if (['running', 'processing', 'active'].includes(norm)) return 'process'
  if (['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(norm)) return 'error'
  return 'wait'
}

const stepProgressPercent = (step: DeployStepView, idx: number, steps: DeployStepView[], overallProgress?: number) => {
  const status = stepStatusToAntd(step.status)
  if (status === 'finish') return 100
  if (status === 'error') return Math.max(5, clampProgress(overallProgress))
  if (status === 'wait') return 0
  const completedBefore = steps.slice(0, idx).filter((item) => stepStatusToAntd(item.status) === 'finish').length
  const stepSpan = 100 / Math.max(steps.length, 1)
  return clampProgress(((overallProgress || 0) - completedBefore * stepSpan) / stepSpan * 100)
}

const majorVersion = (version?: string) => {
  const major = Number(String(version || '').split('.')[0])
  return Number.isFinite(major) ? major : 0
}

const versionSupportsArch = (arch: ArchType, version?: string) => {
  if (arch === 'mgr') return majorVersion(version) >= 8
  return true
}

const ClusterDeploy: React.FC = () => {
  const [hosts, setHosts] = useState<Host[]>([])
  const [instances, setInstances] = useState<Instance[]>([])
  const [versions, setVersions] = useState<VersionEntry[]>([])
  const [tab, setTab] = useState<ArchType>('ha')
  const [submitting, setSubmitting] = useState(false)
  const [historyLoading, setHistoryLoading] = useState(false)
  const [deployments, setDeployments] = useState<DeployResult[]>([])
  const [statusFilter, setStatusFilter] = useState<string[]>(['success'])
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
  const [pendingDeployPayload, setPendingDeployPayload] = useState<any>(null)
  const [pendingDeployArch, setPendingDeployArch] = useState<ArchType>('ha')
  const [pendingDeployValues, setPendingDeployValues] = useState<any>(null)

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

  const normalizeDeployment = (data: any): DeployResult => ({
    deployment_id: data.deployment_id || data.id,
    cluster_id: data.cluster_id || data.deployment_id || data.id,
    cluster_type: data.cluster_type,
    status: data.status || 'pending',
    progress: deploymentProgress(data.status, data.progress),
    stage: data.stage,
    message: data.message || '',
    mysql_user: data.mysql_user,
    mysql_password: data.mysql_password,
    started_at: data.started_at || data.created_at,
    finished_at: data.finished_at || data.updated_at,
    nodes: Array.isArray(data.nodes) ? data.nodes : [],
    steps: Array.isArray(data.steps) ? data.steps : [],
    logs: Array.isArray(data.logs) ? data.logs : [],
  })

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

  const deploymentNodes = (record: DeployResult) => {
    if (record.nodes?.length) {
      return record.nodes.map((node) => {
        const endpoint = `${node.host || '-'}:${node.port || '-'}`
        return `${node.name || node.instance_id || '-'} (${endpoint}, ${formatClusterRole(record.cluster_type, node.role)})`
      })
    }
    const clusterID = record.cluster_id || record.deployment_id
    return instances
      .filter((inst) => inst.cluster_id === clusterID)
      .map((inst) => {
        const endpoint = `${inst.connection?.host || inst.host || '-'}:${inst.connection?.port || inst.port || '-'}`
        const role = formatClusterRole(record.cluster_type, inst.status?.role)
        return `${inst.name} (${endpoint}, ${role})`
      })
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
        const stepIdx = next.stage ? STAGE_ORDER.indexOf(next.stage) : -1
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
    const payload = buildDeployPayload(arch, nextValues)
    setPlanPreviewLoading(true)
    setPlanPreviewArch(arch)
    clusterDeployApi.validateCluster(payload).then((res: any) => {
      const plan = res?.data?.plan || res?.data
      setPlanPreviewData(plan)
      setPendingDeployPayload(payload)
      setPendingDeployArch(arch)
      setPendingDeployValues(nextValues)
      setPlanPreviewOpen(true)
    }).catch((err: any) => {
      message.error(`计划验证失败: ${err?.response?.data?.message || err?.message}`)
    }).finally(() => {
      setPlanPreviewLoading(false)
    })
  }

  const doConfirmDeploy = () => {
    if (!pendingDeployPayload || !pendingDeployArch || !pendingDeployValues) return
    setPlanPreviewOpen(false)
    doDeploy(pendingDeployArch, pendingDeployValues, pendingDeployPayload)
  }

  const runDeploy = (arch: ArchType, values: any) => {
    // Show plan preview before deployment
    doPreview(arch, values)
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
    const payload = buildDeployPayload(arch, nextValues)
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
      setPendingDeployPayload(null)
      setPlanPreviewOpen(true)
    } catch (err: any) {
      message.error(`获取部署计划失败: ${err?.response?.data?.message || err?.message}`)
    }
  }

  const doDeploy = async (arch: ArchType, values: any, payloadOverride?: any) => {
    setSubmitting(true)
    setCurrentStep(0)
    setActiveDeployment(null)
    const payload = payloadOverride || buildDeployPayload(arch, values)
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

  const parseMySQLConfig = (text?: string) => {
    const config: Record<string, string> = {}
    ;(text || '').split('\n').forEach((line) => {
      const item = line.trim()
      if (!item || item.startsWith('#')) return
      const idx = item.indexOf('=')
      if (idx <= 0) return
      config[item.slice(0, idx).trim()] = item.slice(idx + 1).trim()
    })
    return config
  }

  const buildDeployPayload = (arch: ArchType, values: any) => {
    const replicaHostIDs = values.replica_host_ids || (values.replica_host_id ? [values.replica_host_id] : [])
    const nodes: any[] = []
    const addNode = (hostID: string, role: string, port?: number, extra?: Record<string, any>) => {
      if (!hostID) return
      nodes.push({
        host_id: hostID,
        role,
        mysql_port: port || values.mysql_port || 3306,
        data_dir: extra?.data_dir,
        basedir: extra?.basedir,
        server_id: extra?.server_id,
        custom: extra?.custom,
        package_url: extra?.package_url,
        relay_url: extra?.relay_url,
      })
    }

    if (arch === 'ha') {
      addNode(values.master_host_id, 'master', values.mysql_port, { data_dir: values.master_data_dir, server_id: values.master_server_id })
      replicaHostIDs.forEach((hostID: string, index: number) => addNode(hostID, 'replica', values.replica_port || values.mysql_port || 3306, {
        data_dir: values.replica_data_dir,
        server_id: values.replica_server_id || (values.master_server_id ? Number(values.master_server_id) + index + 1 : undefined),
      }))
    } else if (arch === 'mha') {
      addNode(values.manager_host_id, 'manager', values.manager_port || values.mysql_port)
      addNode(values.master_host_id, 'master', values.mysql_port)
      replicaHostIDs.forEach((hostID: string) => addNode(hostID, 'replica', values.replica_port || values.mysql_port || 3306))
    } else if (arch === 'mgr') {
      addNode(values.master_host_id, 'primary', values.mysql_port, { server_id: values.master_server_id, custom: { local_port: values.local_port } })
      replicaHostIDs.forEach((hostID: string, index: number) => addNode(hostID, 'secondary', values.replica_port || values.mysql_port || 3306, {
        data_dir: values.replica_data_dir,
        server_id: values.replica_server_id || (values.master_server_id ? Number(values.master_server_id) + index + 1 : undefined),
        custom: values.local_port ? { local_port: Number(values.local_port) + index + 1 } : undefined,
      }))
    } else {
      addNode(values.master_host_id, 'bootstrap', values.mysql_port, { data_dir: values.master_data_dir })
      replicaHostIDs.forEach((hostID: string) => addNode(hostID, 'secondary', values.replica_port || values.mysql_port || 3306, { data_dir: values.replica_data_dir }))
    }

    const custom: Record<string, any> = {}
    // Auto-inject relay_url from system settings if configured
    try {
      const relayCfg = localStorage.getItem('dbops_relay_server')
      if (relayCfg) {
        const parsed = JSON.parse(relayCfg)
        const platformIp = window.location.hostname || '10.3.67.52'
        const defaultVersion = parsed.default_version || values.mysql_version || '8.0.36'
        const resolveVars = (url: string) => {
          const parts = defaultVersion.split('.')
          const majorMinor = parts.length >= 2 ? `${parts[0]}.${parts[1]}` : defaultVersion
          return url
            .replace(/\$\{platform_ip\}/g, platformIp)
            .replace(/\$\{version\}/g, defaultVersion)
            .replace(/\$\{major_minor\}/g, majorMinor)
            .replace(/\$\{major\}/g, parts[0] || defaultVersion)
            .replace(/\$\{minor\}/g, parts[1] || '')
        }
        // Build relay_url from sources (new format) or legacy relay_url (old format)
        let relayUrl = ''
        if (parsed.sources && parsed.sources.length > 0) {
          const firstEnabled = parsed.sources.find((s: any) => s.enabled && s.url)
          if (firstEnabled) relayUrl = resolveVars(firstEnabled.url).replace(/\/+$/, '')
        } else if (parsed.relay_url) {
          relayUrl = resolveVars(parsed.relay_url).replace(/\/+$/, '')
        }
        if (relayUrl && parsed.relay_path) {
          relayUrl += '/' + parsed.relay_path.replace(/^\/+/, '').replace(/\/+$/, '')
        }
        if (relayUrl) custom.relay_url = relayUrl
        custom.relay_upload_url = window.location.origin + '/api/v1/relay/upload'
      }
    } catch { /* ignore */ }

    // Propagate relay_url to each node for precheck repair
    if (custom.relay_url) {
      for (const node of nodes) {
        if (!node.relay_url) node.relay_url = custom.relay_url
      }
    }

    if (arch === 'ha' && values.semi_sync_enabled !== undefined) custom.semi_sync_enabled = !!values.semi_sync_enabled
    if (arch === 'mha') {
      if (values.vip) custom.vip = values.vip
      if (values.vip_interface) custom.vip_interface = values.vip_interface
      if (values.ping_interval) custom.ping_interval = values.ping_interval
      if (values.ping_retry) custom.ping_retry = values.ping_retry
      if (values.ssh_user) custom.ssh_user = values.ssh_user
    }
    if (arch === 'mgr') {
      if (values.group_name) custom.group_name = values.group_name
      if (values.local_port) custom.local_port = values.local_port
    }
    if (arch === 'pxc') {
      if (values.wsrep_port) custom.wsrep_port = values.wsrep_port
      if (values.cluster_name) custom.cluster_name = values.cluster_name
      if (values.sst_method) custom.sst_method = values.sst_method
      if (values.wsrep_sst_port) custom.wsrep_sst_port = values.wsrep_sst_port
      if (values.wsrep_ssl_enabled !== undefined) custom.wsrep_ssl_enabled = !!values.wsrep_ssl_enabled
    }

    return {
      cluster_id: values.cluster_id,
      name: values.cluster_id,
      cluster_type: arch,
      mode: values.pseudo_mode ? 'pseudo' : 'real',
      mysql: {
        version: values.mysql_version || '8.0',
        user: credential.username,
        password: credential.password,
        package_url: values.package_url,
        package_checksum: values.package_checksum,
        config: parseMySQLConfig(values.mysql_config_text),
      },
      replication: {
        user: values.repl_user,
        password: values.repl_password,
        mode: arch === 'mgr' ? 'single-primary' : arch === 'pxc' ? 'galera' : 'async',
      },
      nodes,
      custom,
    }
  }

  const columns: ColumnsType<DeployResult> = [
    { title: '部署ID', dataIndex: 'deployment_id', key: 'deployment_id', width: 180, ellipsis: true },
    { title: '集群ID', dataIndex: 'cluster_id', key: 'cluster_id', width: 150, ellipsis: true },
    {
      title: '架构',
      dataIndex: 'cluster_type',
      key: 'cluster_type',
      width: 80,
      render: (type: ArchType) => <Tag color={type === 'ha' ? 'cyan' : type === 'mha' ? 'blue' : type === 'mgr' ? 'green' : 'orange'}>{type.toUpperCase()}</Tag>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 110,
      render: (status: string) => {
        if (isCompletedDeployStatus(status)) return <Tag color="success" icon={<CheckCircleOutlined />}>成功</Tag>
        if (isDestroyedDeployStatus(status)) return <Tag color="default">已销毁</Tag>
        if (isPartialDeployStatus(status)) return <Tag color="warning" icon={<CloseCircleOutlined />}>部分完成</Tag>
        if (isFailedDeployStatus(status)) return <Tag color="error" icon={<CloseCircleOutlined />}>失败</Tag>
        if (normalizeStatus(status) === 'pending') return <Tag color="default">待开始</Tag>
        return <Tag color="processing" icon={<ReloadOutlined spin />}>进行中</Tag>
      },
    },
    { title: '当前阶段', dataIndex: 'stage', key: 'stage', width: 100, ellipsis: true, render: (stage: string) => stage || '-' },
    {
      title: '进度',
      dataIndex: 'progress',
      key: 'progress',
      width: 140,
      render: (progress: number, record) => <Progress percent={deploymentProgress(record.status, progress)} size="small" status={deploymentProgressStatus(record.status)} />,
    },
    { title: '信息', dataIndex: 'message', key: 'message', ellipsis: true },
    {
      title: '节点信息',
      key: 'nodes',
      width: 200,
      ellipsis: true,
      render: (_, record) => {
        const nodes = deploymentNodes(record)
        if (nodes.length === 0) return '-'
        return (
          <Space direction="vertical" size={2}>
            {nodes.map((node) => <span key={node}>{node}</span>)}
          </Space>
        )
      },
    },
    { title: '开始时间', dataIndex: 'started_at', key: 'started_at', width: 160, render: (time: string) => (time ? new Date(time).toLocaleString() : '-') },
    {
      title: '操作',
      key: 'action',
      width: 140,
      render: (_, record) => (
        <Space>
          <Button size="small" icon={<EyeOutlined />} onClick={() => viewDeployPlan(record)}>
            查看计划
          </Button>
          <Button size="small" danger icon={<DeleteOutlined />} disabled={isDestroyedDeployStatus(record.status)} onClick={() => destroyDeployment(record)}>
            销毁
          </Button>
        </Space>
      ),
    },
  ]

  const renderForm = (
    arch: ArchType,
    form: any,
    extraFields: React.ReactNode,
    onFinish: (values: any) => void,
    options?: { simpleReplica?: boolean },
  ) => (
    <Form form={form} layout="horizontal" onFinish={onFinish}>
      <Form.Item name="pseudo_mode" valuePropName="checked" initialValue={false} hidden>
        <Checkbox />
      </Form.Item>
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
                  label: `${v.flavor} ${v.version}${v.is_lts ? ' [LTS]' : ''}${v.min_glibc ? ` (glibc>=${v.min_glibc})` : ''}`,
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
        <div style={{ marginTop: 16 }}>
          <strong>环境预检结果</strong>
          <Table
            size="small"
            pagination={false}
            style={{ marginTop: 8 }}
            dataSource={precheckResults}
            rowKey="host_id"
            columns={[
              { title: '主机', dataIndex: 'host', key: 'host' },
              {
                title: '状态',
                dataIndex: 'status',
                key: 'status',
                render: (s: string) => (
                  <Tag color={s === 'pass' ? 'success' : s === 'warn' ? 'warning' : 'error'}>
                    {s === 'pass' ? '通过' : s === 'warn' ? '警告' : '失败'}
                  </Tag>
                ),
              },
              { title: '消息', dataIndex: 'message', key: 'message' },
              {
                title: '详情',
                dataIndex: 'details',
                key: 'details',
                render: (details: any[], record: any) => (
                  <Space direction="vertical" size={2}>
                    {(details || []).map((d, i) => (
                      <span key={i} style={{ fontSize: 12 }}>
                        <Tag color={d.passed ? 'success' : 'error'} style={{ fontSize: 10 }}>{d.name}</Tag>
                        {d.value && <span style={{ color: '#888' }}>{d.value} </span>}
                        {d.message && <span style={{ color: '#ff4d4f' }}>{d.message}</span>}
                        {!d.passed && d.fixable && (
                          <Button
                            size="small"
                            type="link"
                            loading={precheckLoading}
                            onClick={() => repairPrecheckItem(record, d)}
                            style={{ padding: '0 4px', height: 20 }}
                          >
                            修复
                          </Button>
                        )}
                      </span>
                    ))}
                  </Space>
                ),
              },
            ]}
          />
        </div>
      )}
    </Form>
  )

  const renderVerticalStepProgress = (steps: DeployStepView[], overallProgress?: number) => (
    <Steps
      direction="vertical"
      size="small"
      current={steps.findIndex((step) => stepStatusToAntd(step.status) === 'process')}
      items={steps.map((step, idx) => {
        const status = stepStatusToAntd(step.status)
        const percent = stepProgressPercent(step, idx, steps, overallProgress)
        return {
          title: (
            <Space size={4} wrap>
              <span>{step.name || step.id || `步骤 ${idx + 1}`}</span>
              {step.type && <Tag color="default" style={{ fontSize: 10 }}>{STEP_TYPE_CN[step.type] || step.type}</Tag>}
              {step.target_node && <span style={{ color: '#888', fontSize: 12 }}>({step.target_node})</span>}
            </Space>
          ),
          description: (
            <Space direction="vertical" size={4} style={{ width: '100%' }}>
              <Progress
                percent={percent}
                size="small"
                status={status === 'error' ? 'exception' : status === 'finish' ? 'success' : 'active'}
              />
              <Space size={8} wrap>
                {step.message && <span style={{ fontSize: 12, color: '#666' }}>{step.message}</span>}
                {step.depends_on && step.depends_on.length > 0 && (
                  <span style={{ fontSize: 12, color: '#888' }}>依赖: {step.depends_on.join(', ')}</span>
                )}
                {step.started_at && <span style={{ fontSize: 11, color: '#aaa' }}>开始 {new Date(step.started_at).toLocaleTimeString()}</span>}
                {step.completed_at && <span style={{ fontSize: 11, color: '#aaa' }}>完成 {new Date(step.completed_at).toLocaleTimeString()}</span>}
              </Space>
            </Space>
          ),
          status,
        }
      })}
    />
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
          <div>
            <Space style={{ marginBottom: 16 }}>
              <Select
                mode="multiple"
                placeholder="筛选状态"
                value={statusFilter}
                onChange={setStatusFilter}
                style={{ minWidth: 200 }}
                maxTagCount="responsive"
                options={[
                  { label: '成功', value: 'success' },
                  { label: '失败', value: 'failed' },
                  { label: '部分完成', value: 'partial' },
                  { label: '运行中', value: 'running' },
                  { label: '待开始', value: 'pending' },
                  { label: '已销毁', value: 'destroyed' },
                ]}
              />
              <Select
                placeholder="筛选架构"
                value={archFilter}
                onChange={setArchFilter}
                style={{ width: 120 }}
                options={[
                  { label: '全部架构', value: 'all' },
                  { label: 'HA', value: 'ha' },
                  { label: 'MHA', value: 'mha' },
                  { label: 'MGR', value: 'mgr' },
                  { label: 'PXC', value: 'pxc' },
                ]}
              />
            </Space>
            {filteredDeployments.length === 0 ? (
              <Empty description="暂无符合条件的部署记录" />
            ) : (
              <Table
                columns={columns}
                dataSource={filteredDeployments}
                rowKey="deployment_id"
                loading={historyLoading}
                scroll={{ x: 'max-content' }}
                pagination={{
                  defaultPageSize: 10,
                  showSizeChanger: true,
                  showQuickJumper: true,
                  showTotal: (t) => `共 ${t} 条记录`,
                }}
              />
            )}
          </div>
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

      {/* Plan Preview Modal */}
      <Modal
        title={
          <Space>
            <EyeOutlined />
            <span>部署计划预览 - {planPreviewArch?.toUpperCase()}</span>
          </Space>
        }
        open={planPreviewOpen}
        onCancel={() => {
          setPlanPreviewOpen(false)
          setPlanPreviewData(null)
        }}
        width={800}
        footer={
          pendingDeployPayload ? (
            <Space>
              <Button onClick={() => {
                setPlanPreviewOpen(false)
                setPlanPreviewData(null)
              }}>取消</Button>
              <Button type="primary" icon={<PlayCircleOutlined />} onClick={doConfirmDeploy}>
                确认部署
              </Button>
            </Space>
          ) : (
            <Button onClick={() => {
              setPlanPreviewOpen(false)
              setPlanPreviewData(null)
            }}>关闭</Button>
          )
        }
        destroyOnClose
      >
        {planPreviewData ? (
          <div>
            {/* Mode warning for new deployments */}
            {pendingDeployPayload && (
              <Alert
                type={pendingDeployValues?.pseudo_mode ? 'info' : 'warning'}
                message={pendingDeployValues?.pseudo_mode
                  ? '伪集群演练只写入平台纳管关系和拓扑，不会修改目标主机上的数据库服务。'
                  : '真实部署会修改目标主机上的 MySQL 实例、复制配置和服务状态。请确认已完成环境检查并具备回滚方案。'}
                style={{ marginBottom: 16 }}
                showIcon
              />
            )}
            {/* Plan Summary */}
            <Descriptions size="small" column={2} bordered style={{ marginBottom: 16 }}>
              <Descriptions.Item label="部署ID">{planPreviewData.deployment_id || planPreviewData.id || '-'}</Descriptions.Item>
              <Descriptions.Item label="架构类型">
                <Tag color={planPreviewData.cluster_type === 'ha' ? 'cyan' : planPreviewData.cluster_type === 'mha' ? 'blue' : planPreviewData.cluster_type === 'mgr' ? 'green' : 'orange'}>
                  {(planPreviewData.cluster_type || '').toUpperCase()}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="部署模式">
                <Tag>{planPreviewData.mode || 'real'}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="节点数量">{planPreviewData.nodes?.length || 0}</Descriptions.Item>
              <Descriptions.Item label="步骤数量">{planPreviewData.steps?.length || 0}</Descriptions.Item>
              {planPreviewData.parameters?.mysql_version && (
                <Descriptions.Item label="MySQL 版本">{planPreviewData.parameters.mysql_version}</Descriptions.Item>
              )}
            </Descriptions>

            {/* Nodes Table */}
            <strong style={{ display: 'block', marginBottom: 8 }}>节点列表</strong>
            <Table
              size="small"
              columns={[
                { title: 'Host', dataIndex: 'host', key: 'host', width: 140 },
                { title: '角色', dataIndex: 'role', key: 'role', width: 100, render: (role: string) => <Tag>{formatClusterRole(planPreviewArch, role)}</Tag> },
                { title: 'MySQL 端口', dataIndex: 'mysql_port', key: 'mysql_port', width: 100 },
                { title: 'Agent 端口', dataIndex: 'agent_port', key: 'agent_port', width: 100 },
                { title: '数据目录', dataIndex: 'data_dir', key: 'data_dir', width: 140, render: (v: string) => v || '-' },
                { title: 'Server ID', dataIndex: 'server_id', key: 'server_id', width: 80, render: (v: number) => v || '-' },
              ]}
              dataSource={planPreviewData.nodes || []}
              rowKey={(row: any, index?: number) => row.id || row.host || `node-${index}`}
              pagination={false}
              style={{ marginBottom: 16 }}
            />

            {/* Steps Timeline */}
            <strong style={{ display: 'block', marginBottom: 8 }}>执行步骤</strong>
            {renderVerticalStepProgress((planPreviewData.steps || []).map((step: any) => ({
              ...step,
              status: step.status || 'planned',
            })))}

            {/* Architecture-specific parameters */}
            {planPreviewData.parameters && Object.keys(planPreviewData.parameters).length > 0 && (
              <div style={{ marginTop: 16 }}>
                <strong style={{ display: 'block', marginBottom: 8 }}>部署参数</strong>
                <Descriptions size="small" column={2} bordered>
                  {Object.entries(planPreviewData.parameters).map(([key, value]: [string, any]) => (
                    <Descriptions.Item label={key} key={key}>
                      {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                    </Descriptions.Item>
                  ))}
                </Descriptions>
              </div>
            )}
          </div>
        ) : (
          <Empty description="无法加载部署计划" />
        )}
      </Modal>

      {/* Credentials Success Modal */}
      <Modal
        title={
          <Space>
            <KeyOutlined />
            <span>部署成功 - MySQL 凭证信息</span>
          </Space>
        }
        open={credentialModalResult.visible}
        onCancel={() => setCredentialModalResult({ visible: false, mysql_user: '', mysql_password: '' })}
        footer={
          <Button type="primary" onClick={() => {
            setCredentialModalResult({ visible: false, mysql_user: '', mysql_password: '' })
          }}>
            我已保存
          </Button>
        }
        width={600}
      >
        <Alert
          type="warning"
          showIcon
          message="请立即保存以下 MySQL 连接信息！此信息关闭后将不再显示。"
          description="部署后的 MySQL root 密码仅在此处展示一次，请保存到安全位置。可在实例详情页通过「强制修改密码」功能重置密码。"
          style={{ marginBottom: 16 }}
        />
        <Descriptions size="small" column={1} bordered style={{ marginBottom: 16 }}>
          <Descriptions.Item label="用户名">{credentialModalResult.mysql_user}</Descriptions.Item>
          <Descriptions.Item label="密码">
            <Text copyable={{ text: credentialModalResult.mysql_password }}>
              {credentialModalResult.mysql_password}
            </Text>
          </Descriptions.Item>
        </Descriptions>
        {credentialModalResult.nodes && credentialModalResult.nodes.length > 0 && (
          <Table
            size="small"
            pagination={false}
            columns={[
              { title: '节点', dataIndex: 'host', key: 'host' },
              { title: '端口', dataIndex: 'port', key: 'port' },
              { title: '角色', dataIndex: 'role', key: 'role', render: (role: string) => formatClusterRole(planPreviewArch, role) },
              { title: '用户名', dataIndex: 'username', key: 'username' },
              {
                title: '密码',
                dataIndex: 'password',
                key: 'password',
                render: (pw: string) => <Text copyable={{ text: pw }}>{pw}</Text>,
              },
            ]}
            dataSource={credentialModalResult.nodes}
            rowKey={(row) => `${row.host}:${row.port}`}
          />
        )}
      </Modal>

      <Modal
        title="部署失败详情"
        open={!!deployErrorDetail}
        onCancel={() => setDeployErrorDetail(null)}
        footer={<Button type="primary" onClick={() => setDeployErrorDetail(null)}>关闭</Button>}
        width={820}
      >
        {deployErrorDetail && (
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Alert
              type="error"
              showIcon
              message={`${deployErrorDetail.cluster_type?.toUpperCase?.() || '集群'} 部署失败`}
              description={<pre style={{ whiteSpace: 'pre-wrap', margin: 0 }}>{deployErrorDetail.message || deployErrorDetail.status}</pre>}
            />
            {deployErrorDetail.steps && deployErrorDetail.steps.length > 0 && (
              <Table
                size="small"
                pagination={false}
                rowKey={(row, index) => `${row.name}-${index}`}
                dataSource={deployErrorDetail.steps}
                columns={[
                  { title: '步骤', dataIndex: 'name', key: 'name', width: 220 },
                  {
                    title: '状态',
                    dataIndex: 'status',
                    key: 'status',
                    width: 90,
                    render: (status: string) => <Tag color={isFailedDeployStatus(status) ? 'error' : stepStatusToAntd(status) === 'finish' ? 'success' : 'default'}>{status}</Tag>,
                  },
                  { title: '信息', dataIndex: 'message', key: 'message', render: (text: string) => text ? <pre style={{ whiteSpace: 'pre-wrap', margin: 0 }}>{text}</pre> : '-' },
                ]}
              />
            )}
          </Space>
        )}
      </Modal>

      {/* MySQL Root Password Modal */}
      <Modal
        title={
          <Space>
            <KeyOutlined />
            <span>设置 MySQL Root 密码</span>
          </Space>
        }
        open={mysqlPasswordModalOpen}
        onCancel={() => setMysqlPasswordModalOpen(false)}
        onOk={handleSaveMysqlPassword}
        okText="保存"
        cancelText="取消"
        destroyOnClose
      >
        <Alert
          type="info"
          showIcon
          message="此密码将用于集群部署时设置 MySQL root 用户密码"
          description="部署过程中会使用此密码初始化 MySQL root 账户。请牢记此密码，部署完成后可使用此密码连接 MySQL。"
          style={{ marginBottom: 16 }}
        />
        <Form form={mysqlPasswordForm} layout="vertical">
          <Form.Item name="username" label="用户名" initialValue="root">
            <Input placeholder="root" disabled />
          </Form.Item>
          <Form.Item
            name="password"
            label="密码"
            rules={[{ required: true, message: '请输入 MySQL root 密码' }]}
            initialValue="Root#1234"
          >
            <Input.Password placeholder="请输入 MySQL root 密码" />
          </Form.Item>
        </Form>
      </Modal>

      {activeDeployment && (
        <Card
          title={
            <Space>
              <ClusterOutlined />
              <span>部署进度 - {activeDeployment.cluster_type?.toUpperCase()}</span>
              {!isTerminalDeployStatus(activeDeployment.status) && (
                <span style={{ fontSize: 12, color: '#1677ff' }}>
                  <ReloadOutlined spin style={{ marginRight: 4 }} />
                  实时更新中 (2s)
                </span>
              )}
            </Space>
          }
          style={{ marginTop: 16 }}
          extra={
            <Space>
              <Tag
                color={
                  isCompletedDeployStatus(activeDeployment.status) ? 'success'
                  : isPartialDeployStatus(activeDeployment.status) ? 'warning'
                  : isFailedDeployStatus(activeDeployment.status) ? 'error'
                  : isDestroyedDeployStatus(activeDeployment.status) ? 'default'
                  : 'processing'
                }
                icon={
                  isCompletedDeployStatus(activeDeployment.status) ? <CheckCircleOutlined />
                  : isFailedDeployStatus(activeDeployment.status) ? <CloseCircleOutlined />
                  : isDestroyedDeployStatus(activeDeployment.status) ? <DeleteOutlined />
                  : !isTerminalDeployStatus(activeDeployment.status) ? <ReloadOutlined spin />
                  : undefined
                }
              >
                {isCompletedDeployStatus(activeDeployment.status) ? '已完成'
                  : isPartialDeployStatus(activeDeployment.status) ? '部分完成'
                  : isFailedDeployStatus(activeDeployment.status) ? '失败'
                  : isDestroyedDeployStatus(activeDeployment.status) ? '已销毁'
                  : '运行中'}
              </Tag>
              {activeDeployment.finished_at && <span>完成于 {new Date(activeDeployment.finished_at).toLocaleString()}</span>}
            </Space>
          }
        >
          {/* Architecture info banner */}
          <div style={{ marginBottom: 16, display: 'flex', alignItems: 'center', gap: 12 }}>
            <Tag color={activeDeployment.cluster_type === 'ha' ? 'cyan' : activeDeployment.cluster_type === 'mha' ? 'blue' : activeDeployment.cluster_type === 'mgr' ? 'green' : 'orange'} style={{ fontSize: 14, padding: '2px 12px' }}>
              <ClusterOutlined style={{ marginRight: 4 }} />
              {activeDeployment.cluster_type?.toUpperCase()}
            </Tag>
            {activeDeployment.nodes && activeDeployment.nodes.length > 0 && (
              <Space size={4}>
                {activeDeployment.nodes.map((node, idx) => (
                  <Tag key={idx} color={node.role === 'master' || node.role === 'primary' || node.role === 'bootstrap' ? 'blue' : node.role === 'manager' ? 'purple' : 'default'}>
                    {formatClusterRole(activeDeployment.cluster_type, node.role)}
                  </Tag>
                ))}
              </Space>
            )}
          </div>

          {/* 5-stage progress bar */}
          <Steps
            current={currentStep}
            size="small"
            items={STAGE_ORDER.map((title, idx) => ({
              title,
              description: idx === currentStep && !isTerminalDeployStatus(activeDeployment.status)
                ? <span style={{ fontSize: 11, color: '#1677ff' }}>进行中...</span>
                : idx < currentStep
                  ? <span style={{ fontSize: 11, color: '#52c41a' }}>已完成</span>
                  : undefined,
            }))}
            status={deploymentStepStatus(activeDeployment.status)}
          />

          {/* Overall progress bar */}
          <div style={{ marginTop: 16, display: 'flex', alignItems: 'center', gap: 12 }}>
            <div style={{ flex: 1 }}>
              <Progress
                percent={deploymentProgress(activeDeployment.status, activeDeployment.progress)}
                status={deploymentProgressStatus(activeDeployment.status)}
                strokeColor={{
                  '0%': '#108ee9',
                  '100%': '#87d068',
                }}
              />
            </div>
            <span style={{ fontSize: 24, fontWeight: 600, color: '#333', minWidth: 48, textAlign: 'right' }}>
              {deploymentProgress(activeDeployment.status, activeDeployment.progress)}%
            </span>
          </div>

          {/* Status message */}
          <Alert
            type={
              isFailedDeployStatus(activeDeployment.status) ? 'error'
              : isCompletedDeployStatus(activeDeployment.status) ? 'success'
              : isPartialDeployStatus(activeDeployment.status) ? 'warning'
              : 'info'
            }
            message={
              <Space>
                <span>{activeDeployment.message || '等待后端返回状态...'}</span>
                {!isTerminalDeployStatus(activeDeployment.status) && (
                  <span style={{ fontSize: 11, color: '#888' }}>
                    ({STAGE_ORDER[currentStep] || activeDeployment.stage || '初始化中'})
                  </span>
                )}
              </Space>
            }
            showIcon
            style={{ marginBottom: 16 }}
          />

          {/* Detailed steps timeline */}
          {activeDeployment.steps && activeDeployment.steps.length > 0 && (
            <div style={{ marginTop: 16 }}>
              <strong style={{ display: 'block', marginBottom: 8 }}>详细步骤</strong>
              {renderVerticalStepProgress(activeDeployment.steps, activeDeployment.progress)}
            </div>
          )}

          {/* Fallback substeps when backend doesn't return steps yet */}
          {(!activeDeployment.steps || activeDeployment.steps.length === 0) && (
            <div style={{ marginTop: 16 }}>
              <strong style={{ display: 'block', marginBottom: 8 }}>当前阶段子步骤</strong>
              <div style={{ marginTop: 8 }}>
                {(DEPLOY_SUBSTEPS[activeDeployment.stage || ''] || DEPLOY_SUBSTEPS['环境检查']).map((substep, idx) => {
                  const substeps = DEPLOY_SUBSTEPS[activeDeployment.stage || ''] || DEPLOY_SUBSTEPS['环境检查']
                  const isLast = idx === substeps.length - 1
                  return (
                    <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4, opacity: isLast && !isCompletedDeployStatus(activeDeployment.status) ? 1 : idx < substeps.length - 1 ? 0.6 : 0.4 }}>
                      {idx < substeps.length - 1 ? (
                        <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 14 }} />
                      ) : !isTerminalDeployStatus(activeDeployment.status) ? (
                        <ReloadOutlined spin style={{ color: '#1677ff', fontSize: 14 }} />
                      ) : (
                        <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 14 }} />
                      )}
                      <span style={{ color: isLast && !isCompletedDeployStatus(activeDeployment.status) ? '#333' : '#999' }}>{substep}</span>
                    </div>
                  )
                })}
              </div>
            </div>
          )}

          {/* Node progress cards */}
          {activeDeployment.nodes && activeDeployment.nodes.length > 0 && (
            <div style={{ marginTop: 16 }}>
              <strong style={{ display: 'block', marginBottom: 8 }}>节点进度 ({activeDeployment.nodes.length})</strong>
              <Row gutter={[12, 12]}>
                {activeDeployment.nodes.map((node, idx) => (
                  <Col span={8} key={idx}>
                    <Card
                      size="small"
                      style={{
                        borderLeft: `3px solid ${
                          node.role === 'master' || node.role === 'primary' || node.role === 'bootstrap' ? '#1677ff'
                          : node.role === 'manager' ? '#722ed1'
                          : '#52c41a'
                        }`,
                      }}
                      title={
                        <Space size={4}>
                          <span style={{ fontSize: 13, fontWeight: 500 }}>{node.name || node.instance_id || `节点 ${idx + 1}`}</span>
                          <span style={{ fontSize: 11, color: '#888' }}>{node.host || '-'}:{node.port || '-'}</span>
                        </Space>
                      }
                      extra={
                        <Tag color={node.role === 'master' || node.role === 'primary' || node.role === 'bootstrap' ? 'blue' : node.role === 'manager' ? 'purple' : 'default'}>
                          {formatClusterRole(activeDeployment.cluster_type, node.role)}
                        </Tag>
                      }
                    >
                      <Space direction="vertical" size={4} style={{ width: '100%' }}>
                        <Space>
                          <span style={{ fontSize: 12, color: '#666' }}>状态: </span>
                          <Tag
                            color={
                              node.status === 'completed' || node.status === 'healthy' ? 'success'
                              : node.status === 'running' || node.status === 'deploying' ? 'processing'
                              : node.status === 'failed' ? 'error'
                              : 'default'
                            }
                            style={{ fontSize: 11 }}
                          >
                            {node.status || 'pending'}
                          </Tag>
                          {node.current_step && (
                            <span style={{ fontSize: 11, color: '#888' }}>{node.current_step}</span>
                          )}
                        </Space>
                        {typeof node.progress === 'number' && (
                          <Progress percent={node.progress} size="small" />
                        )}
                        {node.message && (
                          <div style={{ color: '#888', fontSize: 11, lineHeight: 1.4 }}>{node.message}</div>
                        )}
                      </Space>
                    </Card>
                  </Col>
                ))}
              </Row>
            </div>
          )}

          {/* Live log viewer */}
          {activeDeployment.logs && activeDeployment.logs.length > 0 && (
            <div style={{ marginTop: 16 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                <strong>部署日志 ({activeDeployment.logs.length})</strong>
                {!isTerminalDeployStatus(activeDeployment.status) && (
                  <span style={{ fontSize: 11, color: '#1677ff' }}>
                    <ReloadOutlined spin style={{ marginRight: 4 }} />
                    实时
                  </span>
                )}
              </div>
              <div
                style={{
                  background: '#1e1e1e',
                  color: '#d4d4d4',
                  padding: 12,
                  borderRadius: 6,
                  maxHeight: 200,
                  overflow: 'auto',
                  fontFamily: '\"Cascadia Code\", \"Fira Code\", \"Consolas\", monospace',
                  fontSize: 12,
                  lineHeight: 1.6,
                }}
              >
                {activeDeployment.logs.map((log, idx) => (
                  <div key={idx} style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                    <span style={{ color: '#888', marginRight: 8 }}>[{idx + 1}]</span>
                    {log.includes('ERROR') || log.includes('failed') || log.includes('错误') ? (
                      <span style={{ color: '#f56c6c' }}>{log}</span>
                    ) : log.includes('completed') || log.includes('成功') ? (
                      <span style={{ color: '#67c23a' }}>{log}</span>
                    ) : (
                      <span>{log}</span>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Restart/retry button for terminal states */}
          {isTerminalDeployStatus(activeDeployment.status) && (
            <div style={{ marginTop: 16, textAlign: 'center' }}>
              <Button
                icon={<ReloadOutlined />}
                onClick={() => {
                  setActiveDeployment(null)
                  stopPolling()
                }}
              >
                返回部署表单
              </Button>
            </div>
          )}
        </Card>
      )}
    </div>
  )
}

export default ClusterDeploy
