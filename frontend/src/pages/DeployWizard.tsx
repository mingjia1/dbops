import React, { useEffect, useMemo, useState } from 'react'
import {
  Alert, Button, Card, Col, Input, Radio, Result, Row, Select,
  Space, Steps, Typography, message,
} from 'antd'
import {
  CheckCircleOutlined, CloudServerOutlined, RocketOutlined, SafetyOutlined,
} from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import {
  clusterDeployApi, hostApi, instanceApi, type Host,
} from '../services/api'
import {
  WIZARD_DEFAULTS,
  WIZARD_TEMPLATES,
  buildHaWizardPayload,
  buildSingleInstanceCreate,
  buildWizardSummary,
  generateRootPassword,
  getWizardTemplate,
  makeWizardName,
  type WizardScenario,
} from '../services/deployWizardTemplates'
import LiveDeployTracker from '../components/LiveDeployTracker'

const { Title, Paragraph, Text } = Typography

const DeployWizard: React.FC = () => {
  const navigate = useNavigate()
  const [step, setStep] = useState(0)
  const [hosts, setHosts] = useState<Host[]>([])
  const [loadingHosts, setLoadingHosts] = useState(true)

  const [scenario, setScenario] = useState<WizardScenario>('dev-single')
  const [selectedHostIds, setSelectedHostIds] = useState<string[]>([])
  const [password, setPassword] = useState(() => generateRootPassword())
  const [port] = useState(WIZARD_DEFAULTS.port)

  const [submitting, setSubmitting] = useState(false)
  const [resultOk, setResultOk] = useState(false)
  const [resultFail, setResultFail] = useState('')
  const [connInfo, setConnInfo] = useState<{ host: string; port: number; user: string; password: string; name: string } | null>(null)
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null)
  const [activeDeploymentId, setActiveDeploymentId] = useState<string | null>(null)
  const [pendingName, setPendingName] = useState('')

  const template = useMemo(() => getWizardTemplate(scenario), [scenario])

  useEffect(() => {
    setLoadingHosts(true)
    hostApi.list(100, 0)
      .then((res: any) => setHosts(res?.data || []))
      .catch(() => setHosts([]))
      .finally(() => setLoadingHosts(false))
  }, [])

  useEffect(() => {
    setSelectedHostIds((ids) => ids.slice(0, template.maxHosts))
  }, [scenario, template.maxHosts])

  const hostOptions = hosts.map((h) => ({
    value: h.id,
    label: `${h.name || h.address} (${h.address})`,
  }))

  const selectedHosts = selectedHostIds
    .map((id) => hosts.find((h) => h.id === id))
    .filter(Boolean) as Host[]

  const canNextFromHosts = selectedHostIds.length >= template.minHosts
    && selectedHostIds.length <= template.maxHosts
    && !!password

  const summaryLines = buildWizardSummary({
    scenario,
    hostLabels: selectedHosts.map((h) => h.address),
    port,
    password,
  })

  const startDeploy = async () => {
    setSubmitting(true)
    setResultOk(false)
    setResultFail('')
    setActiveTaskId(null)
    setActiveDeploymentId(null)
    setConnInfo(null)
    setStep(3)
    try {
      if (template.mode === 'single') {
        const host = selectedHosts[0]
        if (!host) throw new Error('请选择主机')
        const name = makeWizardName(scenario)
        setPendingName(name)
        const createBody = buildSingleInstanceCreate({
          hostId: host.id,
          hostAddress: host.address,
          password,
          name,
          port,
        })
        const created: any = await instanceApi.create(createBody)
        const instanceId = created?.data?.id || created?.data?.ID || created?.id
        if (!instanceId) throw new Error('创建实例未返回 ID')
        const deployRes: any = await instanceApi.deploy(instanceId)
        const taskId = deployRes?.data?.task_id || deployRes?.task_id
        if (!taskId) {
          // 兼容旧同步：直接成功/失败
          const st = String(deployRes?.data?.status || deployRes?.status || '').toLowerCase()
          if (['failed', 'error'].includes(st)) {
            throw new Error(deployRes?.data?.message || '单机部署失败')
          }
          setResultOk(true)
          setConnInfo({
            host: host.address, port, user: WIZARD_DEFAULTS.username, password, name,
          })
          message.success('部署成功')
          return
        }
        setActiveTaskId(taskId)
        setConnInfo({
          host: host.address, port, user: WIZARD_DEFAULTS.username, password, name,
        })
      } else {
        if (selectedHosts.length < 2) throw new Error('HA 至少需要 2 台主机')
        const master = selectedHosts[0]
        const replicas = selectedHosts.slice(1)
        const clusterId = makeWizardName('prod-ha')
        setPendingName(clusterId)
        const payload = buildHaWizardPayload({
          masterHostId: master.id,
          replicaHostIds: replicas.map((h) => h.id),
          password,
          clusterId,
          port,
        })
        const res: any = await clusterDeployApi.deployCluster(payload)
        const deploymentId = res?.data?.deployment_id || res?.data?.id
        if (!deploymentId) throw new Error('未返回 deployment_id')
        setActiveDeploymentId(deploymentId)
        setConnInfo({
          host: master.address, port, user: WIZARD_DEFAULTS.username, password, name: clusterId,
        })
      }
    } catch (e: any) {
      const msg = e?.response?.data?.message || e?.message || '部署失败'
      setResultFail(msg)
      message.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  const onTrackerTerminal = (ok: boolean, msg?: string) => {
    if (ok) {
      setResultOk(true)
      setResultFail('')
      message.success('部署成功')
    } else {
      setResultFail(msg || '部署失败')
      setResultOk(false)
      message.error(msg || '部署失败')
    }
  }

  return (
    <div className="apple-page apple-fade-in" style={{ maxWidth: 960, margin: '0 auto' }}>
      <Title level={3} style={{ marginBottom: 4 }}>
        <RocketOutlined /> 部署数据库向导
      </Title>
      <Paragraph type="secondary">
        选场景 → 选机器 → 确认 → 实时看安装过程。高级选项请用「集群部署（高级）」。
      </Paragraph>

      <Steps
        current={step}
        style={{ marginBottom: 24 }}
        items={[
          { title: '选场景' },
          { title: '选机器' },
          { title: '确认' },
          { title: '进度' },
        ]}
      />

      {step === 0 && (
        <Row gutter={[16, 16]}>
          {WIZARD_TEMPLATES.map((t) => (
            <Col xs={24} md={8} key={t.id}>
              <Card
                hoverable
                onClick={() => setScenario(t.id)}
                style={{
                  borderColor: scenario === t.id ? '#1677ff' : undefined,
                  boxShadow: scenario === t.id ? '0 0 0 2px rgba(22,119,255,.2)' : undefined,
                  height: '100%',
                }}
              >
                <Space direction="vertical" size={8}>
                  <Radio checked={scenario === t.id}>{t.title}</Radio>
                  <Text type="secondary">{t.desc}</Text>
                  <Text><CloudServerOutlined /> {t.summary}</Text>
                </Space>
              </Card>
            </Col>
          ))}
          <Col span={24}>
            <Space>
              <Button type="primary" onClick={() => setStep(1)}>下一步</Button>
              <Button type="link" onClick={() => navigate('/dashboard/cluster-deploy')}>高级集群部署</Button>
            </Space>
          </Col>
        </Row>
      )}

      {step === 1 && (
        <Card loading={loadingHosts}>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message={
              template.mode === 'single'
                ? '请选择 1 台已录入且可连通的主机（需已装 Agent）'
                : '请选择至少 2 台主机：第一台为主库，其余为从库'
            }
          />
          {hosts.length === 0 ? (
            <Result
              status="info"
              title="还没有主机"
              subTitle="请先添加空主机并安装 Agent"
              extra={<Button type="primary" onClick={() => navigate('/dashboard/hosts/new')}>去添加主机</Button>}
            />
          ) : (
            <>
              <div style={{ marginBottom: 12 }}>
                <Text strong>主机</Text>
                <Select
                  mode={template.maxHosts > 1 ? 'multiple' : undefined}
                  style={{ width: '100%', marginTop: 8 }}
                  placeholder="选择主机"
                  options={hostOptions}
                  value={template.maxHosts > 1 ? selectedHostIds : selectedHostIds[0]}
                  onChange={(v) => {
                    if (Array.isArray(v)) setSelectedHostIds(v.slice(0, template.maxHosts))
                    else setSelectedHostIds(v ? [v as string] : [])
                  }}
                />
              </div>
              <div style={{ marginBottom: 16 }}>
                <Text strong>root 密码</Text>
                <Space.Compact style={{ width: '100%', marginTop: 8 }}>
                  <Input.Password value={password} onChange={(e) => setPassword(e.target.value)} />
                  <Button onClick={() => setPassword(generateRootPassword())}>重新生成</Button>
                </Space.Compact>
              </div>
              <Space>
                <Button onClick={() => setStep(0)}>上一步</Button>
                <Button type="primary" disabled={!canNextFromHosts} onClick={() => setStep(2)}>下一步</Button>
              </Space>
            </>
          )}
        </Card>
      )}

      {step === 2 && (
        <Card>
          <Alert type="warning" showIcon icon={<SafetyOutlined />} message="请确认以下信息，提交后将在选中主机安装 MySQL" style={{ marginBottom: 16 }} />
          <ul>
            {summaryLines.map((line) => (
              <li key={line}><Text>{line}</Text></li>
            ))}
          </ul>
          {template.mode === 'ha' && (
            <Paragraph type="secondary">主库：{selectedHosts[0]?.address}；从库：{selectedHosts.slice(1).map((h) => h.address).join(', ')}</Paragraph>
          )}
          <Space>
            <Button onClick={() => setStep(1)}>上一步</Button>
            <Button type="primary" loading={submitting} onClick={startDeploy}>开始部署</Button>
          </Space>
        </Card>
      )}

      {step === 3 && (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          {(activeTaskId || activeDeploymentId) && (
            <LiveDeployTracker
              taskId={activeTaskId}
              deploymentId={activeDeploymentId}
              title={pendingName ? `安装进度 · ${pendingName}` : '安装进度'}
              onTerminal={onTrackerTerminal}
            />
          )}
          {!activeTaskId && !activeDeploymentId && !resultOk && !resultFail && (
            <Card loading>正在提交部署任务…</Card>
          )}

          {resultOk && connInfo && (
            <Result
              status="success"
              title="数据库已就绪"
              subTitle="请保存连接信息（密码只展示这一次）"
              icon={<CheckCircleOutlined />}
              extra={[
                <Button type="primary" key="inst" onClick={() => navigate('/dashboard/instances')}>查看实例</Button>,
                <Button key="again" onClick={() => {
                  setStep(0); setResultOk(false); setConnInfo(null)
                  setActiveTaskId(null); setActiveDeploymentId(null)
                  setPassword(generateRootPassword())
                }}>再部署一套</Button>,
              ]}
            >
              <Card size="small" style={{ textAlign: 'left', maxWidth: 480, margin: '0 auto' }}>
                <p><Text strong>名称：</Text>{connInfo.name}</p>
                <p><Text strong>地址：</Text>{connInfo.host}:{connInfo.port}</p>
                <p><Text strong>用户：</Text>{connInfo.user}</p>
                <p><Text strong>密码：</Text><Text code copyable>{connInfo.password}</Text></p>
                <p><Text type="secondary">JDBC：</Text>
                  <Text code copyable>{`jdbc:mysql://${connInfo.host}:${connInfo.port}/`}</Text>
                </p>
              </Card>
            </Result>
          )}
          {resultFail && (
            <Result
              status="error"
              title="部署失败"
              subTitle={resultFail}
              extra={[
                <Button key="back" onClick={() => setStep(2)}>返回修改</Button>,
                <Button key="adv" onClick={() => navigate('/dashboard/cluster-deploy')}>去高级部署排查</Button>,
              ]}
            />
          )}
        </Space>
      )}
    </div>
  )
}

export default DeployWizard
