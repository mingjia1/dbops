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
      setInstances(list)

      const ids = Array.from(new Set(list.map((i) => i.cluster_id).filter(Boolean))) as string[]
      const loaded: Record<string, ClusterGraph> = {}
      await Promise.all(ids.map(async (clusterId) => {
        try {
          const [topologyRes, graphRes]: any[] = await Promise.all([
            topologyApi.getCluster(clusterId),
            topologyApi.getGraph(clusterId),
          ])
          loaded[clusterId] = {
            clusterId,
            mode: topologyRes?.data?.replication_mode || graphRes?.data?.edges?.[0]?.label || 'unknown',
            nodes: graphRes?.data?.nodes || [],
            edges: graphRes?.data?.edges || [],
          }
        } catch {
          loaded[clusterId] = { clusterId, mode: 'unknown', nodes: [], edges: [] }
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
              const clusterInstances = instances.filter((i) => i.cluster_id === clusterId)
              const nodes = graph?.nodes?.length ? graph.nodes : clusterInstances.map((i) => ({
                id: i.id,
                name: i.name,
                role: i.status?.role || 'unknown',
                status: i.status?.health_status || i.status?.run_status || 'unknown',
                cluster_id: clusterId,
              }))
              return (
                <Card key={clusterId} size="small" title={<Space><ApartmentOutlined />{clusterId}<Tag>{graph?.mode || 'unknown'}</Tag></Space>}>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12, marginBottom: 12 }}>
                    {nodes.map((node) => (
                      <div key={node.id} style={{ width: 220, minHeight: 92, border: '1px solid #d9d9d9', borderRadius: 6, padding: 12 }}>
                        <Space direction="vertical" size={4}>
                          <Text strong>{node.name || node.id}</Text>
                          <Space>
                            <Tag color={roleColor(node.role)}>{node.role || 'unknown'}</Tag>
                            <Tag color={statusColor(node.status)}>{node.status || 'unknown'}</Tag>
                          </Space>
                          <Text type="secondary" style={{ fontSize: 12 }}>{node.id}</Text>
                        </Space>
                      </div>
                    ))}
                  </div>
                  <Table
                    size="small"
                    pagination={false}
                    columns={[
                      { title: '源节点', dataIndex: 'source_id', key: 'source_id' },
                      { title: '目标节点', dataIndex: 'target_id', key: 'target_id' },
                      { title: '关系', dataIndex: 'label', key: 'label', render: (v) => <Tag>{v || 'replication'}</Tag> },
                    ]}
                    dataSource={(graph?.edges || []).map((edge, index) => ({ ...edge, key: `${edge.source_id}-${edge.target_id}-${index}` }))}
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
