import React, { useEffect, useMemo, useState } from 'react'
import { Button, Card, Col, Empty, Row, Select, Space, Spin, Statistic, Table, Tag, Typography } from 'antd'
import { ApartmentOutlined, DatabaseOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { instanceApi, topologyApi, type Instance } from '../services/api'

const { Text } = Typography

interface TopologyNode {
  id: string
  name: string
  role: string
  status: string
  cluster_id: string
}

interface TopologyEdge {
  source_id: string
  target_id: string
  type: string
  label: string
}

interface ClusterGraph {
  clusterId: string
  mode: string
  nodes: TopologyNode[]
  edges: TopologyEdge[]
}

const primaryRoles = new Set(['master', 'primary', 'primary_master'])

const roleColor = (role?: string) => {
  if (!role) return 'default'
  if (primaryRoles.has(role)) return 'blue'
  if (role === 'slave' || role === 'replica' || role === 'secondary') return 'green'
  return 'default'
}

const statusColor = (status?: string) => {
  if (status === 'healthy' || status === 'running') return 'success'
  if (status === 'failed' || status === 'stopped') return 'error'
  return 'default'
}

const parseSlaveIds = (value?: string) => {
  if (!value) return []
  try {
    const parsed = JSON.parse(value)
    return Array.isArray(parsed) ? parsed.filter(Boolean).map(String) : []
  } catch {
    return value.split(',').map((item) => item.trim()).filter(Boolean)
  }
}

const inferEdgesFromInstances = (clusterInstances: Instance[]): TopologyEdge[] => {
  const ids = new Set(clusterInstances.map((item) => item.id))
  const edgeKeys = new Set<string>()
  const edges: TopologyEdge[] = []
  const addEdge = (sourceId?: string, targetId?: string, label?: string) => {
    if (!sourceId || !targetId || !ids.has(sourceId) || !ids.has(targetId)) return
    const key = `${sourceId}->${targetId}`
    if (edgeKeys.has(key)) return
    edgeKeys.add(key)
    edges.push({
      source_id: sourceId,
      target_id: targetId,
      type: 'replication',
      label: label || 'replication',
    })
  }
  clusterInstances.forEach((instance) => {
    const mode = instance.topology?.replication_mode || instance.status?.replication_status || 'replication'
    addEdge(instance.topology?.master_id, instance.id, mode)
    parseSlaveIds(instance.topology?.slave_ids).forEach((slaveId) => addEdge(instance.id, slaveId, mode))
  })
  if (edges.length > 0 || clusterInstances.length <= 1) return edges

  const primary = clusterInstances.find((instance) => primaryRoles.has(instance.status?.role || '')) || clusterInstances[0]
  clusterInstances.forEach((instance) => {
    if (instance.id !== primary.id) {
      addEdge(primary.id, instance.id, primary.topology?.replication_mode || instance.topology?.replication_mode || 'replication')
    }
  })
  return edges
}

const TopologyView: React.FC = () => {
  const [instances, setInstances] = useState<Instance[]>([])
  const [graphs, setGraphs] = useState<Record<string, ClusterGraph>>({})
  const [loading, setLoading] = useState(false)
  const [clusterFilter, setClusterFilter] = useState<string | undefined>()

  const clusterIds = useMemo(
    () => Array.from(new Set(instances.map((i) => i.cluster_id).filter(Boolean))) as string[],
    [instances],
  )

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await instanceApi.list(1000, 0)
      const list: Instance[] = res?.data || []

      // 只保留有集群ID的实例（过滤掉已销毁集群的残留实例）
      const validInstances = list.filter((i) => i.cluster_id && i.cluster_id.trim() !== '')
      setInstances(validInstances)

      const ids = Array.from(new Set(validInstances.map((i) => i.cluster_id).filter(Boolean))) as string[]
      const loaded: Record<string, ClusterGraph> = {}

      await Promise.all(ids.map(async (clusterId) => {
        try {
          const [topologyRes, graphRes]: any[] = await Promise.all([
            topologyApi.getCluster(clusterId),
            topologyApi.getGraph(clusterId),
          ])

          // 验证返回的图数据是否有效
          const nodes = graphRes?.data?.nodes || []
          const edges = graphRes?.data?.edges || []

          // 如果图为空，可能集群已被销毁，跳过
          if (nodes.length === 0) {
            console.warn(`Cluster ${clusterId} has no topology nodes, possibly destroyed`)
            return
          }

          loaded[clusterId] = {
            clusterId,
            mode: topologyRes?.data?.replication_mode || edges?.[0]?.label || 'async',
            nodes,
            edges,
          }
        } catch (error) {
          console.warn(`Failed to load topology for cluster ${clusterId}:`, error)
          // 不添加到loaded中，避免显示空/错误的拓扑
        }
      }))

      setGraphs(loaded)
    } catch (error) {
      console.error('Failed to fetch topology data:', error)
      setInstances([])
      setGraphs({})
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
  }, [])

  const visibleClusterIds = clusterFilter ? [clusterFilter] : clusterIds
  const visibleInstances = clusterFilter ? instances.filter((i) => i.cluster_id === clusterFilter) : instances
  const standalones = instances.filter((i) => !i.cluster_id)

  const instanceColumns: ColumnsType<Instance> = [
    { title: '实例', dataIndex: 'name', key: 'name' },
    {
      title: '地址',
      key: 'endpoint',
      render: (_, item) => `${item.connection?.host || item.host || '-'}:${item.connection?.port || item.port || '-'}`,
    },
    {
      title: '角色',
      key: 'role',
      render: (_, item) => <Tag color={roleColor(item.status?.role)}>{item.status?.role || 'unknown'}</Tag>,
    },
    {
      title: '健康',
      key: 'health',
      render: (_, item) => <Tag color={statusColor(item.status?.health_status)}>{item.status?.health_status || '-'}</Tag>,
    },
  ]

  const renderTopologyGraph = (nodes: TopologyNode[], edges: TopologyEdge[]) => {
    if (nodes.length === 0) {
      return (
        <div style={{ padding: 24, textAlign: 'center', color: '#999' }}>
          <DatabaseOutlined style={{ fontSize: 48, marginBottom: 12 }} />
          <div>暂无拓扑节点</div>
        </div>
      )
    }

    const width = 920
    const height = Math.max(180, Math.ceil(nodes.length / 4) * 150)
    const positions = new Map<string, { x: number; y: number }>()
    let primaryIndex = 0
    let replicaIndex = 0

    nodes.forEach((node) => {
      const isPrimary = primaryRoles.has(node.role)
      const slot = isPrimary ? primaryIndex++ : replicaIndex++
      const x = isPrimary ? 90 + (slot % 2) * 180 : 360 + (slot % 3) * 180
      const y = isPrimary ? 70 + Math.floor(slot / 2) * 130 : 70 + Math.floor(slot / 3) * 130
      positions.set(node.id, { x, y })
    })

    return (
      <div style={{ width: '100%', overflowX: 'auto', marginBottom: 16 }}>
        <svg width={width} height={height} style={{ minWidth: width, border: '1px solid #f0f0f0', borderRadius: 6, background: 'var(--page-bg, #fff)' }}>
          <defs>
            <marker id="topology-arrow" markerWidth="10" markerHeight="10" refX="9" refY="3" orient="auto" markerUnits="strokeWidth">
              <path d="M0,0 L0,6 L9,3 z" fill="#8c8c8c" />
            </marker>
          </defs>
          {edges.map((edge, index) => {
            const source = positions.get(edge.source_id)
            const target = positions.get(edge.target_id)
            if (!source || !target) return null
            return (
              <g key={`${edge.source_id}-${edge.target_id}-${index}`}>
                <line
                  x1={source.x + 82}
                  y1={source.y + 30}
                  x2={target.x - 12}
                  y2={target.y + 30}
                  stroke="#8c8c8c"
                  strokeWidth={2}
                  markerEnd="url(#topology-arrow)"
                />
                <text x={(source.x + target.x) / 2 + 30} y={(source.y + target.y) / 2 + 22} fontSize="12" fill="#595959">
                  {edge.label || 'async'}
                </text>
              </g>
            )
          })}
          {nodes.map((node) => {
            const pos = positions.get(node.id) || { x: 0, y: 0 }
            const isPrimary = primaryRoles.has(node.role)
            const color = isPrimary ? '#1677ff' : '#52c41a'
            const isHealthy = node.status === 'healthy' || node.status === 'running'
            const borderColor = isHealthy ? color : '#ff4d4f'

            return (
              <g key={node.id} transform={`translate(${pos.x}, ${pos.y})`}>
                <rect width="150" height="64" rx="6" fill="#fff" stroke={borderColor} strokeWidth="2" />
                <text x="12" y="24" fontSize="13" fontWeight="600" fill="#262626">{node.name || node.id}</text>
                <text x="12" y="45" fontSize="12" fill={color}>{node.role || 'unknown'}</text>
                <text x="12" y="60" fontSize="11" fill={isHealthy ? '#52c41a' : '#ff4d4f'}>{node.status || 'unknown'}</text>
              </g>
            )
          })}
        </svg>
      </div>
    )
  }

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={<Space><ApartmentOutlined /><span>拓扑视图</span></Space>}
        extra={
          <Space>
            <Select
              placeholder="按集群筛选"
              allowClear
              style={{ width: 260 }}
              value={clusterFilter}
              onChange={setClusterFilter}
              options={clusterIds.map((id) => ({ value: id, label: id }))}
            />
            <Button icon={<ReloadOutlined />} onClick={fetchData}>刷新</Button>
          </Space>
        }
      >
        <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
          <Col xs={12} md={6}><Statistic title="实例数" value={visibleInstances.length} prefix={<DatabaseOutlined />} /></Col>
          <Col xs={12} md={6}><Statistic title="集群数" value={visibleClusterIds.length} prefix={<ApartmentOutlined />} /></Col>
          <Col xs={12} md={6}><Statistic title="主节点" value={visibleInstances.filter((i) => primaryRoles.has(i.status?.role || '')).length} /></Col>
          <Col xs={12} md={6}><Statistic title="独立实例" value={standalones.length} /></Col>
        </Row>

        {loading ? (
          <div style={{ textAlign: 'center', padding: 48 }}><Spin /></div>
        ) : visibleInstances.length === 0 ? (
          <Empty description="暂无拓扑数据" />
        ) : (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            {visibleClusterIds.map((clusterId) => {
              const graph = graphs[clusterId]

              // 如果图数据不存在或为空，跳过该集群（可能已被销毁）
              if (!graph || (graph.nodes.length === 0 && instances.filter((i) => i.cluster_id === clusterId).length === 0)) {
                return null
              }

              const clusterInstances = instances.filter((i) => i.cluster_id === clusterId)

              // 如果没有实例且没有图数据，跳过
              if (clusterInstances.length === 0 && graph.nodes.length === 0) {
                return null
              }

              const nodes = graph?.nodes?.length ? graph.nodes : clusterInstances.map((i) => ({
                id: i.id,
                name: i.name,
                role: i.status?.role || 'unknown',
                status: i.status?.health_status || i.status?.run_status || 'unknown',
                cluster_id: clusterId,
              }))
              const edges = graph?.edges?.length ? graph.edges : inferEdgesFromInstances(clusterInstances)

              // 计算集群健康状态
              const healthyCount = nodes.filter((n) => n.status === 'healthy' || n.status === 'running').length
              const primaryCount = nodes.filter((n) => primaryRoles.has(n.role)).length
              const replicaCount = nodes.length - primaryCount
              const clusterHealth = healthyCount === nodes.length ? 'success' : healthyCount > 0 ? 'warning' : 'error'

              return (
                <Card
                  key={clusterId}
                  size="small"
                  title={
                    <Space>
                      <ApartmentOutlined />
                      <span>{clusterId}</span>
                      <Tag color="blue">{graph?.mode || 'async'}</Tag>
                      <Tag color={clusterHealth}>
                        {healthyCount}/{nodes.length} 健康
                      </Tag>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        {primaryCount}主 {replicaCount}从
                      </Text>
                    </Space>
                  }
                >
                  {renderTopologyGraph(nodes, edges)}
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12, marginBottom: 12 }}>
                    {nodes.map((node) => {
                      const isHealthy = node.status === 'healthy' || node.status === 'running'
                      return (
                        <div
                          key={node.id}
                          style={{
                            width: 220,
                            minHeight: 92,
                            border: `1px solid ${isHealthy ? '#d9d9d9' : '#ff7875'}`,
                            borderRadius: 6,
                            padding: 12,
                            backgroundColor: isHealthy ? '#fff' : '#fff2f0',
                          }}
                        >
                          <Space direction="vertical" size={4}>
                            <Text strong>{node.name || node.id}</Text>
                            <Space>
                              <Tag color={roleColor(node.role)}>{node.role || 'unknown'}</Tag>
                              <Tag color={statusColor(node.status)}>{node.status || 'unknown'}</Tag>
                            </Space>
                            <Text type="secondary" style={{ fontSize: 12 }}>{node.id}</Text>
                          </Space>
                        </div>
                      )
                    })}
                  </div>
                  <Table
                    size="small"
                    pagination={false}
                    columns={[
                      { title: '源节点', dataIndex: 'source_id', key: 'source_id' },
                      { title: '目标节点', dataIndex: 'target_id', key: 'target_id' },
                      { title: '复制模式', dataIndex: 'label', key: 'label', render: (v) => <Tag>{v || 'async'}</Tag> },
                    ]}
                    dataSource={edges.map((edge, index) => ({ ...edge, key: `${edge.source_id}-${edge.target_id}-${index}` }))}
                    locale={{ emptyText: '暂无复制关系' }}
                  />
                </Card>
              )
            })}

            {standalones.length > 0 && !clusterFilter && (
              <Card size="small" title="独立实例">
                <Table columns={instanceColumns} dataSource={standalones} rowKey="id" pagination={false} />
              </Card>
            )}
          </Space>
        )}
      </Card>
    </div>
  )
}

export default TopologyView
