import React, { useEffect, useMemo, useState } from 'react'
import { Button, Card, Col, Empty, Row, Select, Space, Spin, Statistic, Table, Tag, Typography } from 'antd'
import { ApartmentOutlined, DatabaseOutlined, ReloadOutlined } from '@ant-design/icons'
import { instanceApi, topologyApi, type Instance } from '../services/api'
import { formatClusterRole, inferArchFromReplicationMode } from '../services/roleDisplay'
import {
  TopologyNode, TopologyEdge, ClusterGraph,
  roleColor, statusColor, endpointOf, relationLabel, parseSlaveIds, inferEdgesFromInstances,
  primaryRoles, replicaRoles,
} from '../services/topologyHelpers'

const { Text } = Typography

const TopologyView: React.FC = () => {
  const [instances, setInstances] = useState<Instance[]>([])
  const [graphs, setGraphs] = useState<Record<string, ClusterGraph>>({})
  const [loading, setLoading] = useState(false)
  const [clusterFilter, setClusterFilter] = useState<string | undefined>()

  const clusterIds = useMemo(
    () => Array.from(new Set(instances.map((item) => item.cluster_id).filter(Boolean))) as string[],
    [instances],
  )

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await instanceApi.list(1000, 0)
      const list: Instance[] = res?.data || []
      const validInstances = list.filter((item) => item.cluster_id && item.cluster_id.trim() !== '')
      setInstances(validInstances)

      const ids = Array.from(new Set(validInstances.map((item) => item.cluster_id).filter(Boolean))) as string[]
      const loaded: Record<string, ClusterGraph> = {}
      await Promise.all(ids.map(async (clusterId) => {
        try {
          const [topologyRes, graphRes]: any[] = await Promise.all([
            topologyApi.getCluster(clusterId),
            topologyApi.getGraph(clusterId),
          ])
          const nodes = graphRes?.data?.nodes || []
          const edges = graphRes?.data?.edges || []
          if (nodes.length === 0) return
          loaded[clusterId] = {
            clusterId,
            mode: topologyRes?.data?.replication_mode || edges?.[0]?.label || 'async',
            nodes,
            edges,
          }
        } catch {
          // A single broken cluster should not hide the remaining topology.
        }
      }))
      setGraphs(loaded)
    } catch {
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
  const visibleInstances = clusterFilter ? instances.filter((item) => item.cluster_id === clusterFilter) : instances

  const renderGraphNode = (node: TopologyNode, instanceByID: Map<string, Instance>, arch?: string) => {
    const instance = instanceByID.get(node.id)
    return (
      <div style={{
        minWidth: 150,
        maxWidth: 180,
        border: '1px solid #d9d9d9',
        borderRadius: 8,
        padding: 10,
        background: '#fff',
        boxShadow: '0 1px 2px rgba(0,0,0,0.04)',
      }}>
        <Space direction="vertical" size={4} style={{ width: '100%' }}>
          <Text strong ellipsis title={node.name || node.id}>{node.name || node.id}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>{endpointOf(instance)}</Text>
          <Space size={4} wrap>
            <Tag color={roleColor(node.role)}>{formatClusterRole(arch, node.role) || 'unknown'}</Tag>
            <Tag color={statusColor(node.status)}>{node.status || 'unknown'}</Tag>
          </Space>
        </Space>
      </div>
    )
  }

  const renderTopologyGraph = (
    nodes: TopologyNode[],
    edges: TopologyEdge[],
    instanceByID: Map<string, Instance>,
    arch?: string,
  ) => {
    if (nodes.length === 0) return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无节点" style={{ margin: '12px 0' }} />
    const nodeByID = new Map(nodes.map((node) => [node.id, node]))
    const linkedTargets = new Set(edges.map((edge) => edge.target_id))
    const roots = nodes.filter((node) => !linkedTargets.has(node.id))
    const startNodes = roots.length > 0 ? roots : nodes.slice(0, 1)
    const rendered = new Set<string>()

    const rows = startNodes.map((root) => {
      const chain: TopologyNode[] = [root]
      rendered.add(root.id)
      edges
        .filter((edge) => edge.source_id === root.id)
        .forEach((edge) => {
          const target = nodeByID.get(edge.target_id)
          if (target) {
            chain.push(target)
            rendered.add(target.id)
          }
        })
      return { root, chain, edges: edges.filter((edge) => edge.source_id === root.id) }
    })

    const detached = nodes.filter((node) => !rendered.has(node.id))

    return (
      <Space direction="vertical" size={16} style={{ width: '100%', overflowX: 'auto', paddingBottom: 8 }}>
        {rows.map((row) => (
          <div key={row.root.id} style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 'max-content' }}>
            {row.chain.map((node, index) => (
              <React.Fragment key={node.id}>
                {index > 0 && (
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6, color: '#8c8c8c' }}>
                    <span style={{ width: 42, height: 1, background: '#bfbfbf', display: 'inline-block' }} />
                    <Tag>{relationLabel(row.edges[index - 1]?.label || row.edges[index - 1]?.type)}</Tag>
                    <span style={{ fontSize: 18, lineHeight: 1 }}>→</span>
                  </div>
                )}
                {renderGraphNode(node, instanceByID, arch)}
              </React.Fragment>
            ))}
          </div>
        ))}
        {detached.length > 0 && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 'max-content' }}>
            {detached.map((node) => renderGraphNode(node, instanceByID, arch))}
          </div>
        )}
      </Space>
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
          <Col xs={12} md={6}><Statistic title="主节点" value={visibleInstances.filter((item) => primaryRoles.has((item.status?.role || '').toLowerCase())).length} /></Col>
          <Col xs={12} md={6}><Statistic title="从节点" value={visibleInstances.filter((item) => replicaRoles.has((item.status?.role || '').toLowerCase())).length} /></Col>
        </Row>

        {loading ? (
          <div style={{ textAlign: 'center', padding: 48 }}><Spin /></div>
        ) : visibleInstances.length === 0 ? (
          <Empty description="暂无拓扑数据" />
        ) : (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            {visibleClusterIds.map((clusterId) => {
              const graph = graphs[clusterId]
              const clusterInstances = instances.filter((item) => item.cluster_id === clusterId)
              if (!graph && clusterInstances.length === 0) return null

              const nodes = graph?.nodes?.length ? graph.nodes : clusterInstances.map((item) => ({
                id: item.id,
                name: item.name,
                role: item.status?.role || 'unknown',
                status: item.status?.health_status || item.status?.run_status || 'unknown',
                cluster_id: clusterId,
              }))
              const edges = graph?.edges?.length ? graph.edges : inferEdgesFromInstances(clusterInstances)
              const instanceByID = new Map(clusterInstances.map((item) => [item.id, item]))
              const nodeByID = new Map(nodes.map((node) => [node.id, node]))
              const primaryNodes = nodes.filter((node) => primaryRoles.has((node.role || '').toLowerCase()))
              const replicaNodes = nodes.filter((node) => replicaRoles.has((node.role || '').toLowerCase()))
              const otherNodes = nodes.filter((node) => !primaryRoles.has((node.role || '').toLowerCase()) && !replicaRoles.has((node.role || '').toLowerCase()))
              const healthyCount = nodes.filter((node) => ['healthy', 'running', 'success'].includes((node.status || '').toLowerCase())).length
              const clusterHealth = healthyCount === nodes.length ? 'success' : healthyCount > 0 ? 'warning' : 'error'
              const arch = inferArchFromReplicationMode(graph?.mode || clusterInstances[0]?.status?.replication_status || clusterInstances[0]?.topology?.replication_mode)

              return (
                <Card
                  key={clusterId}
                  size="small"
                  title={
                    <Space wrap>
                      <ApartmentOutlined />
                      <span>{clusterId}</span>
                      <Tag color="blue">{graph?.mode || 'async'}</Tag>
                      <Tag color={clusterHealth}>{healthyCount}/{nodes.length} 健康</Tag>
                      <Text type="secondary">{primaryNodes.length} 主 / {replicaNodes.length} 从 / {otherNodes.length} 其他</Text>
                    </Space>
                  }
                >
                  {renderTopologyGraph(nodes, edges, instanceByID, arch)}
                  <Table
                    size="small"
                    pagination={false}
                    columns={[
                      { title: '源节点', dataIndex: 'source_id', key: 'source_id', render: (id) => nodeByID.get(id)?.name || id },
                      { title: '目标节点', dataIndex: 'target_id', key: 'target_id', render: (id) => nodeByID.get(id)?.name || id },
                      { title: '关系', dataIndex: 'label', key: 'label', render: (value, row) => <Tag>{relationLabel(value || row.type)}</Tag> },
                    ]}
                    dataSource={edges.map((edge, index) => ({ ...edge, key: `${edge.source_id}-${edge.target_id}-${index}` }))}
                    locale={{ emptyText: '暂无复制关系' }}
                  />
                </Card>
              )
            })}
          </Space>
        )}
      </Card>
    </div>
  )
}

export default TopologyView
