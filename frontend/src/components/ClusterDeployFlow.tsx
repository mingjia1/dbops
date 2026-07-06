import React, { useMemo, useState } from 'react'
import {
  Alert, Button, Checkbox, Col, Drawer, Form, Input, InputNumber, Modal, Row, Segmented, Select, Space, Tag, Typography, message,
} from 'antd'
import { EyeOutlined, PlayCircleOutlined, SettingOutlined } from '@ant-design/icons'
import { Background, Controls, MiniMap, ReactFlow, type Edge, type Node } from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type { Host, VersionEntry } from '../services/api'
import type { ArchType, DeployResult } from '../services/deployHelpers'
import { deploymentPayloadFingerprint, versionSupportsArch } from '../services/deployHelpers'
import {
  createDefaultFlowSpec,
  deployPlanToFlowElements,
  enabledMiddleware,
  flowSpecToDeployPayload,
  middlewareByID,
  roleLabel,
  toolByID,
  validateFlowSpec,
  type DeployFlowDbNode,
  type DeployFlowMiddlewareNode,
  type DeployFlowSpec,
  type DeployFlowToolNode,
} from '../services/deployFlowHelpers'

const { Text } = Typography

type FlowNodeData = {
  [key: string]: unknown
  label: React.ReactNode
  status?: string
  subtitle?: string
}

interface ClusterDeployFlowProps {
  hosts: Host[]
  versions: VersionEntry[]
  credential: { username: string; password: string }
  activeDeployment?: DeployResult | null
  plan?: any
  planFingerprint?: string
  planLoading?: boolean
  submitting?: boolean
  onPreview: (arch: ArchType, payload: Record<string, any>) => Promise<void> | void
  onDeploy: (arch: ArchType, payload: Record<string, any>) => Promise<void> | void
}

const archOptions: Array<{ label: string; value: ArchType }> = [
  { label: 'HA', value: 'ha' },
  { label: 'MHA', value: 'mha' },
  { label: 'MGR', value: 'mgr' },
  { label: 'PXC', value: 'pxc' },
]

const middlewareLabels: Record<DeployFlowMiddlewareNode['id'], string> = {
  keepalived: 'Keepalived',
  proxysql: 'ProxySQL',
}

const toolLabels: Record<DeployFlowToolNode['id'], string> = {
  precheck: 'Precheck',
  health_check: 'Health check',
  baseline_backup: 'Baseline backup',
}

const ClusterDeployFlow: React.FC<ClusterDeployFlowProps> = ({
  hosts,
  versions,
  credential,
  activeDeployment,
  plan,
  planFingerprint,
  planLoading,
  submitting,
  onPreview,
  onDeploy,
}) => {
  const [spec, setSpec] = useState<DeployFlowSpec>(() => createDefaultFlowSpec('ha'))
  const [selectedNodeID, setSelectedNodeID] = useState<string | null>(null)
  const [drawerOpen, setDrawerOpen] = useState(false)

  const hostOptions = useMemo(() => hosts.map((host) => ({
    value: host.id,
    label: `${host.name} (${host.address})${host.agent_status ? ` - ${host.agent_status}` : ''}`,
  })), [hosts])

  const versionOptions = useMemo(() => versions
    .slice()
    .filter((item) => versionSupportsArch(spec.arch, item.version))
    .sort((a, b) => {
      if (a.flavor !== b.flavor) return a.flavor.localeCompare(b.flavor)
      return b.release_date.localeCompare(a.release_date)
    })
    .map((item) => ({
      value: item.version,
      label: `${item.flavor} ${item.version}${item.is_lts ? ' [LTS]' : ''}${item.local_available ? ' [local]' : ' [remote]'}`,
    })), [spec.arch, versions])

  const errors = useMemo(() => validateFlowSpec(spec), [spec])
  const selectedDbNode = selectedNodeID ? spec.nodes.find((node) => node.id === selectedNodeID) : undefined
  const selectedMiddleware = selectedNodeID ? spec.middleware.find((node) => node.id === selectedNodeID) : undefined
  const selectedTool = selectedNodeID ? spec.tools.find((node) => node.id === selectedNodeID) : undefined
  const currentFingerprint = useMemo(() => {
    if (!credential.password || errors.length > 0) return ''
    try {
      return deploymentPayloadFingerprint(flowSpecToDeployPayload(spec, credential))
    } catch {
      return ''
    }
  }, [credential, errors.length, spec])
  const planMatchesCurrentSpec = !planFingerprint || (currentFingerprint !== '' && planFingerprint === currentFingerprint)
  const graphPlan = planMatchesCurrentSpec && plan?.steps?.length && (!plan.cluster_type || plan.cluster_type === spec.arch)
    ? plan
    : (activeDeployment?.cluster_type === spec.arch && activeDeployment?.steps?.length ? { steps: activeDeployment.steps } : null)
  const elements = useMemo(() => {
    if (graphPlan) return deployPlanToFlowElements(graphPlan, activeDeployment || undefined)
    return specToFlowElements(spec, activeDeployment || undefined)
  }, [activeDeployment, graphPlan, spec])

  const reactFlowNodes: Node<FlowNodeData>[] = useMemo(() => elements.nodes.map((node) => {
    const data = (node.data || {}) as Record<string, unknown>
    return {
      id: node.id,
      position: node.position || { x: 0, y: 0 },
      data: {
        label: (
          <FlowNodeLabel
            label={String(data.label || node.id)}
            status={String(data.status || '')}
            subtitle={String(data.type || data.target_node || '')}
          />
        ),
        status: String(data.status || ''),
      },
      style: {
        width: 190,
        minHeight: 64,
        border: '1px solid #d9d9d9',
        borderRadius: 8,
        padding: 10,
        background: '#fff',
        boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
        ...node.style,
      },
    }
  }), [elements.nodes])

  const reactFlowEdges: Edge[] = useMemo(() => elements.edges.map((edge) => ({
    id: edge.id,
    source: edge.source || '',
    target: edge.target || '',
    animated: 'animated' in edge ? edge.animated : false,
  })).filter((edge) => edge.source && edge.target), [elements.edges])

  const updateSpec = (patch: Partial<DeployFlowSpec>) => {
    setSpec((current) => ({ ...current, ...patch }))
  }

  const updateCustom = (patch: Record<string, unknown>) => {
    setSpec((current) => ({ ...current, custom: { ...(current.custom || {}), ...patch } }))
  }

  const updateDbNode = (nodeID: string, patch: Partial<DeployFlowDbNode>) => {
    setSpec((current) => ({
      ...current,
      nodes: current.nodes.map((node) => (node.id === nodeID ? { ...node, ...patch } : node)),
    }))
  }

  const updateMiddleware = (nodeID: DeployFlowMiddlewareNode['id'], patch: Partial<DeployFlowMiddlewareNode>) => {
    setSpec((current) => ({
      ...current,
      middleware: current.middleware.map((node) => (node.id === nodeID ? { ...node, ...patch } : node)),
    }))
  }

  const updateTool = (nodeID: DeployFlowToolNode['id'], patch: Partial<DeployFlowToolNode>) => {
    setSpec((current) => ({
      ...current,
      tools: current.tools.map((node) => (node.id === nodeID ? { ...node, ...patch } : node)),
    }))
  }

  const buildPayload = () => {
    if (!credential.password) {
      message.error('Set the MySQL root password first')
      return null
    }
    try {
      return flowSpecToDeployPayload(spec, credential)
    } catch (err: any) {
      message.error(err?.message || 'Flow validation failed')
      return null
    }
  }

  const handlePreview = async () => {
    const payload = buildPayload()
    if (!payload) return
    await onPreview(spec.arch, payload)
  }

  const handleDeploy = () => {
    const payload = buildPayload()
    if (!payload) return
    Modal.confirm({
      title: `Deploy ${spec.cluster_id}?`,
      content: 'The current flow will be converted into a deployment plan and submitted.',
      okText: 'Deploy',
      cancelText: 'Cancel',
      onOk: () => onDeploy(spec.arch, payload),
    })
  }

  const handleNodeClick = (_: React.MouseEvent, node: Node<FlowNodeData>) => {
    if (graphPlan) return
    const id = node.id
    if (!spec.nodes.some((item) => item.id === id) && !spec.middleware.some((item) => item.id === id) && !spec.tools.some((item) => item.id === id)) {
      return
    }
    setSelectedNodeID(id)
    setDrawerOpen(true)
  }

  const resetArch = (arch: ArchType) => {
    setSpec(createDefaultFlowSpec(arch))
    setSelectedNodeID(null)
    setDrawerOpen(false)
  }

  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(min(100%, 360px), 1fr))', gap: 16, maxWidth: '100%', overflowX: 'hidden' }}>
      <div style={{ minWidth: 0 }}>
        <Space direction="vertical" size={14} style={{ width: '100%' }}>
          <Segmented
            block
            value={spec.arch}
            options={archOptions}
            onChange={(value) => resetArch(value as ArchType)}
          />

          <Form layout="vertical" style={{ maxWidth: '100%' }}>
            <Row gutter={12}>
              <Col span={24}>
                <Form.Item label="Cluster ID" required>
                  <Input value={spec.cluster_id} onChange={(event) => updateSpec({ cluster_id: event.target.value })} />
                </Form.Item>
              </Col>
              <Col span={24}>
                <Form.Item label="MySQL version" required>
                  <Select
                    showSearch
                    optionFilterProp="label"
                    value={spec.mysql_version}
                    options={versionOptions}
                    onChange={(value) => updateSpec({ mysql_version: value })}
                  />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="Default port">
                  <InputNumber min={1} max={65535} style={{ width: '100%' }} value={spec.mysql_port} onChange={(value) => updateSpec({ mysql_port: value || 3306 })} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="Replication user">
                  <Input value={spec.repl_user} onChange={(event) => updateSpec({ repl_user: event.target.value })} />
                </Form.Item>
              </Col>
              <Col span={24}>
                <Form.Item label="Replication password">
                  <Input.Password value={spec.repl_password} onChange={(event) => updateSpec({ repl_password: event.target.value })} />
                </Form.Item>
              </Col>
              {spec.arch === 'mgr' && (
                <Col span={24}>
                  <Form.Item label="MGR group name">
                    <Input value={String(spec.custom?.group_name || '')} onChange={(event) => updateCustom({ group_name: event.target.value })} />
                  </Form.Item>
                </Col>
              )}
              {spec.arch === 'pxc' && (
                <>
                  <Col span={12}>
                    <Form.Item label="wsrep port">
                      <InputNumber min={1} max={65535} style={{ width: '100%' }} value={Number(spec.custom?.wsrep_port || 4567)} onChange={(value) => updateCustom({ wsrep_port: value || 4567 })} />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item label="SST method">
                      <Select
                        value={String(spec.custom?.sst_method || 'xtrabackup-v2')}
                        options={[{ value: 'xtrabackup-v2', label: 'xtrabackup-v2' }, { value: 'mariabackup', label: 'mariabackup' }]}
                        onChange={(value) => updateCustom({ sst_method: value })}
                      />
                    </Form.Item>
                  </Col>
                </>
              )}
            </Row>
          </Form>

          <div>
            <Text strong>Database nodes</Text>
            <Space direction="vertical" size={8} style={{ width: '100%', marginTop: 8 }}>
              {spec.nodes.map((node) => (
                <div key={node.id} style={rowBoxStyle}>
                  <Space direction="vertical" size={6} style={{ width: '100%' }}>
                    <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                      <Text>{roleLabel(node.role)}</Text>
                      <Button size="small" icon={<SettingOutlined />} onClick={() => { setSelectedNodeID(node.id); setDrawerOpen(true) }} />
                    </Space>
                    <Select
                      value={node.host_id}
                      options={hostOptions}
                      placeholder="Select host"
                      onChange={(value) => updateDbNode(node.id, { host_id: value })}
                    />
                  </Space>
                </div>
              ))}
            </Space>
          </div>

          <div>
            <Text strong>Middleware</Text>
            <Space direction="vertical" size={8} style={{ width: '100%', marginTop: 8 }}>
              <Checkbox
                checked={enabledMiddleware(spec, 'keepalived')}
                disabled={spec.arch === 'mgr' || spec.arch === 'pxc'}
                onChange={(event) => updateMiddleware('keepalived', { enabled: event.target.checked })}
              >
                Keepalived
              </Checkbox>
              {enabledMiddleware(spec, 'keepalived') && (
                <Row gutter={8}>
                  <Col span={14}>
                    <Input placeholder="VIP" value={middlewareByID(spec, 'keepalived')?.vip} onChange={(event) => updateMiddleware('keepalived', { vip: event.target.value })} />
                  </Col>
                  <Col span={10}>
                    <Input placeholder="Interface" value={middlewareByID(spec, 'keepalived')?.vip_interface} onChange={(event) => updateMiddleware('keepalived', { vip_interface: event.target.value })} />
                  </Col>
                </Row>
              )}
              <Checkbox
                checked={enabledMiddleware(spec, 'proxysql')}
                onChange={(event) => updateMiddleware('proxysql', { enabled: event.target.checked })}
              >
                ProxySQL
              </Checkbox>
              {enabledMiddleware(spec, 'proxysql') && (
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <Select
                    value={middlewareByID(spec, 'proxysql')?.proxy_host_id}
                    options={hostOptions}
                    placeholder="ProxySQL host"
                    onChange={(value) => {
                      const host = hosts.find((item) => item.id === value)
                      updateMiddleware('proxysql', { proxy_host_id: value, proxy_agent_port: host?.agent_port || 9090 })
                    }}
                  />
                  <Row gutter={8}>
                    <Col span={12}>
                      <InputNumber min={1} max={65535} style={{ width: '100%' }} value={middlewareByID(spec, 'proxysql')?.proxy_port || 6033} onChange={(value) => updateMiddleware('proxysql', { proxy_port: value || 6033 })} />
                    </Col>
                    <Col span={12}>
                      <InputNumber min={1} max={65535} style={{ width: '100%' }} value={middlewareByID(spec, 'proxysql')?.proxy_agent_port || 9090} onChange={(value) => updateMiddleware('proxysql', { proxy_agent_port: value || 9090 })} />
                    </Col>
                  </Row>
                </Space>
              )}
            </Space>
          </div>

          <div>
            <Text strong>Tools</Text>
            <Space direction="vertical" size={8} style={{ width: '100%', marginTop: 8 }}>
              {spec.tools.map((tool) => (
                <Checkbox key={tool.id} checked={tool.enabled} onChange={(event) => updateTool(tool.id, { enabled: event.target.checked })}>
                  {toolLabels[tool.id]}
                </Checkbox>
              ))}
            </Space>
          </div>

          {errors.length > 0 && (
            <Alert type="warning" showIcon message="Flow is incomplete" description={errors.join('; ')} />
          )}

          <Space wrap>
            <Button icon={<EyeOutlined />} loading={planLoading} onClick={handlePreview}>
              Preview plan
            </Button>
            <Button type="primary" icon={<PlayCircleOutlined />} loading={submitting} onClick={handleDeploy}>
              Submit deployment
            </Button>
          </Space>
        </Space>
      </div>

      <div style={{ minWidth: 0, height: 560, border: '1px solid #f0f0f0', borderRadius: 8, overflow: 'hidden' }}>
        <ReactFlow
          nodes={reactFlowNodes}
          edges={reactFlowEdges}
          fitView
          minZoom={0.35}
          nodesDraggable={!graphPlan}
          nodesConnectable={false}
          onNodeClick={handleNodeClick}
        >
          <MiniMap pannable zoomable />
          <Controls />
          <Background gap={18} />
        </ReactFlow>
      </div>

      <Drawer
        title={selectedDbNode ? `${roleLabel(selectedDbNode.role)} node settings` : selectedMiddleware ? `${middlewareLabels[selectedMiddleware.id]} settings` : selectedTool ? `${toolLabels[selectedTool.id]} settings` : 'Node settings'}
        open={drawerOpen}
        width={420}
        onClose={() => setDrawerOpen(false)}
      >
        {selectedDbNode && (
          <Form layout="vertical">
            <Form.Item label="Host" required>
              <Select value={selectedDbNode.host_id} options={hostOptions} onChange={(value) => updateDbNode(selectedDbNode.id, { host_id: value })} />
            </Form.Item>
            <Form.Item label="MySQL port">
              <InputNumber min={1} max={65535} style={{ width: '100%' }} value={selectedDbNode.mysql_port || spec.mysql_port} onChange={(value) => updateDbNode(selectedDbNode.id, { mysql_port: value || spec.mysql_port })} />
            </Form.Item>
            <Form.Item label="Data directory">
              <Input placeholder={`/data/mysql/${selectedDbNode.mysql_port || spec.mysql_port}`} value={selectedDbNode.data_dir} onChange={(event) => updateDbNode(selectedDbNode.id, { data_dir: event.target.value })} />
            </Form.Item>
            <Form.Item label="Base directory">
              <Input placeholder={`/usr/local/mysql-${selectedDbNode.mysql_port || spec.mysql_port}`} value={selectedDbNode.basedir} onChange={(event) => updateDbNode(selectedDbNode.id, { basedir: event.target.value })} />
            </Form.Item>
            <Form.Item label="server_id">
              <InputNumber min={1} style={{ width: '100%' }} value={selectedDbNode.server_id} onChange={(value) => updateDbNode(selectedDbNode.id, { server_id: value || undefined })} />
            </Form.Item>
            {spec.arch === 'mgr' && selectedDbNode.role !== 'manager' && (
              <Form.Item label="Group communication port">
                <InputNumber
                  min={1}
                  max={65535}
                  style={{ width: '100%' }}
                  value={Number(selectedDbNode.custom?.local_port || 33061)}
                  onChange={(value) => updateDbNode(selectedDbNode.id, { custom: { ...(selectedDbNode.custom || {}), local_port: value || 33061 } })}
                />
              </Form.Item>
            )}
          </Form>
        )}
        {selectedMiddleware && selectedMiddleware.id === 'keepalived' && (
          <Form layout="vertical">
            <Form.Item>
              <Checkbox checked={selectedMiddleware.enabled} disabled={spec.arch === 'mgr' || spec.arch === 'pxc'} onChange={(event) => updateMiddleware('keepalived', { enabled: event.target.checked })}>
                Enable Keepalived
              </Checkbox>
            </Form.Item>
            <Form.Item label="VIP">
              <Input value={selectedMiddleware.vip} onChange={(event) => updateMiddleware('keepalived', { vip: event.target.value })} />
            </Form.Item>
            <Form.Item label="VIP interface">
              <Input value={selectedMiddleware.vip_interface} onChange={(event) => updateMiddleware('keepalived', { vip_interface: event.target.value })} />
            </Form.Item>
          </Form>
        )}
        {selectedMiddleware && selectedMiddleware.id === 'proxysql' && (
          <Form layout="vertical">
            <Form.Item>
              <Checkbox checked={selectedMiddleware.enabled} onChange={(event) => updateMiddleware('proxysql', { enabled: event.target.checked })}>
                Enable ProxySQL
              </Checkbox>
            </Form.Item>
            <Form.Item label="ProxySQL host">
              <Select
                value={selectedMiddleware.proxy_host_id}
                options={hostOptions}
                onChange={(value) => {
                  const host = hosts.find((item) => item.id === value)
                  updateMiddleware('proxysql', { proxy_host_id: value, proxy_agent_port: host?.agent_port || 9090 })
                }}
              />
            </Form.Item>
            <Form.Item label="ProxySQL service port">
              <InputNumber min={1} max={65535} style={{ width: '100%' }} value={selectedMiddleware.proxy_port || 6033} onChange={(value) => updateMiddleware('proxysql', { proxy_port: value || 6033 })} />
            </Form.Item>
            <Form.Item label="Agent port">
              <InputNumber min={1} max={65535} style={{ width: '100%' }} value={selectedMiddleware.proxy_agent_port || 9090} onChange={(value) => updateMiddleware('proxysql', { proxy_agent_port: value || 9090 })} />
            </Form.Item>
          </Form>
        )}
        {selectedTool && (
          <Form layout="vertical">
            <Form.Item>
              <Checkbox checked={selectedTool.enabled} onChange={(event) => updateTool(selectedTool.id, { enabled: event.target.checked })}>
                Enable {toolLabels[selectedTool.id]}
              </Checkbox>
            </Form.Item>
            {selectedTool.id === 'baseline_backup' && (
              <Form.Item label="Backup type">
                <Select
                  value={selectedTool.backup_type || 'full'}
                  options={[{ value: 'full', label: 'full' }, { value: 'incremental', label: 'incremental' }]}
                  onChange={(value) => updateTool('baseline_backup', { backup_type: value })}
                />
              </Form.Item>
            )}
          </Form>
        )}
      </Drawer>
    </div>
  )
}

const rowBoxStyle: React.CSSProperties = {
  border: '1px solid #f0f0f0',
  borderRadius: 8,
  padding: 10,
  background: '#fff',
}

const FlowNodeLabel: React.FC<{ label: string; status?: string; subtitle?: string }> = ({ label, status, subtitle }) => (
  <Space direction="vertical" size={4} style={{ width: '100%' }}>
    <Space style={{ justifyContent: 'space-between', width: '100%' }} align="start">
      <Text strong style={{ whiteSpace: 'normal', wordBreak: 'break-word' }}>{label}</Text>
      {status && <Tag color={statusColor(status)} style={{ marginInlineEnd: 0 }}>{status}</Tag>}
    </Space>
    {subtitle && <Text type="secondary" style={{ fontSize: 12 }}>{subtitle}</Text>}
  </Space>
)

const specToFlowElements = (spec: DeployFlowSpec, deployment?: Partial<DeployResult>) => {
  const statusByID = deploymentStepStatusMap(deployment)
  const nodes = [
    flowNode(`arch-${spec.arch}`, spec.arch.toUpperCase(), 'architecture', { x: 0, y: 170 }, deployment?.status),
  ]
  const edges: ReturnType<typeof flowEdge>[] = []
  let lastID = `arch-${spec.arch}`

  const precheck = toolByID(spec, 'precheck')
  if (precheck) {
    nodes.push(flowNode(precheck.id, toolLabels.precheck, 'tool', { x: 250, y: 40 }, statusByID.get('tool_precheck') || enabledStatus(precheck.enabled)))
    if (precheck.enabled) {
      edges.push(flowEdge(lastID, precheck.id))
      lastID = precheck.id
    }
  }

  spec.nodes.forEach((node, index) => {
    const status = statusByID.get(node.id) || nodeStatusByTarget(deployment, node.id) || 'planned'
    nodes.push(flowNode(node.id, roleLabel(node.role), selectedHostLabel(node.host_id), { x: 250, y: 150 + index * 105 }, status))
    edges.push(flowEdge(lastID, node.id))
    lastID = node.id
  })

  spec.middleware.forEach((node, index) => {
    const status = statusByID.get(`middleware_${node.id}`) || enabledStatus(node.enabled)
    nodes.push(flowNode(node.id, middlewareLabels[node.id], node.enabled ? 'middleware' : 'disabled', { x: 520, y: 150 + index * 105 }, status))
    if (node.enabled) {
      edges.push(flowEdge(lastID, node.id))
      lastID = node.id
    }
  })

  spec.tools.filter((tool) => tool.id !== 'precheck').forEach((tool, index) => {
    const status = statusByID.get(`tool_${tool.id}`) || enabledStatus(tool.enabled)
    nodes.push(flowNode(tool.id, toolLabels[tool.id], tool.enabled ? 'tool' : 'disabled', { x: 790, y: 150 + index * 105 }, status))
    if (tool.enabled) {
      edges.push(flowEdge(lastID, tool.id))
      lastID = tool.id
    }
  })

  return { nodes, edges }
}

const flowNode = (id: string, label: string, subtitle: string, position: { x: number; y: number }, status?: string) => ({
  id,
  position,
  data: { label, status: status || 'planned', type: subtitle },
  style: nodeStyle(status),
})

const flowEdge = (source: string, target: string) => ({
  id: `${source}->${target}`,
  source,
  target,
})

const enabledStatus = (enabled: boolean) => (enabled ? 'enabled' : 'disabled')

const selectedHostLabel = (hostID?: string) => hostID || 'No host selected'

const deploymentStepStatusMap = (deployment?: Partial<DeployResult>) => {
  const map = new Map<string, string>()
  ;(deployment?.steps || []).forEach((step) => {
    if (step.id) map.set(step.id, step.status || 'planned')
    if (step.target_node) map.set(step.target_node, step.status || 'planned')
  })
  return map
}

const nodeStatusByTarget = (deployment: Partial<DeployResult> | undefined, nodeID: string) => {
  const steps = deployment?.steps?.filter((step) => step.target_node === nodeID) || []
  if (steps.some((step) => ['failed', 'error'].includes(String(step.status || '').toLowerCase()))) return 'failed'
  if (steps.some((step) => ['running', 'process'].includes(String(step.status || '').toLowerCase()))) return 'running'
  if (steps.length > 0 && steps.every((step) => ['completed', 'success'].includes(String(step.status || '').toLowerCase()))) return 'completed'
  return undefined
}

const nodeStyle = (status?: string): Record<string, unknown> => {
  const normalized = String(status || '').toLowerCase()
  if (['completed', 'success', 'enabled'].includes(normalized)) return { borderColor: '#52c41a', background: '#f6ffed' }
  if (['failed', 'error'].includes(normalized)) return { borderColor: '#ff4d4f', background: '#fff2f0' }
  if (['running', 'process'].includes(normalized)) return { borderColor: '#1677ff', background: '#e6f4ff' }
  if (['disabled'].includes(normalized)) return { borderColor: '#d9d9d9', background: '#fafafa', opacity: 0.72 }
  if (['partial', 'partial_success'].includes(normalized)) return { borderColor: '#faad14', background: '#fffbe6' }
  return { borderColor: '#d9d9d9', background: '#fff' }
}

const statusColor = (status?: string) => {
  const normalized = String(status || '').toLowerCase()
  if (['completed', 'success', 'enabled'].includes(normalized)) return 'green'
  if (['failed', 'error'].includes(normalized)) return 'red'
  if (['running', 'process'].includes(normalized)) return 'blue'
  if (['partial', 'partial_success'].includes(normalized)) return 'gold'
  return 'default'
}

export default ClusterDeployFlow
