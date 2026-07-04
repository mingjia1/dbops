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
import { TopologyGraph } from '../components/TopologyGraph'

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
                  <TopologyGraph nodes={nodes} edges={edges} instanceByID={instanceByID} arch={arch} />
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
