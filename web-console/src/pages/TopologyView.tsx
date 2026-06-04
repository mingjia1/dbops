import React, { useEffect, useState } from 'react'
import { Card, Select, Space, Tag, Empty, Spin, Row, Col, Statistic, Button } from 'antd'
import { ApartmentOutlined, ReloadOutlined, DatabaseOutlined } from '@ant-design/icons'
import { instanceApi, type Instance } from '../services/api'

interface ClusterTopology {
  cluster_id: string
  master: Instance | null
  replicas: Instance[]
}

const TopologyView: React.FC = () => {
  const [instances, setInstances] = useState<Instance[]>([])
  const [loading, setLoading] = useState(false)
  const [clusterFilter, setClusterFilter] = useState<string | undefined>(undefined)

  const fetchData = async () => {
    setLoading(true)
    try {
      const res: any = await instanceApi.list(100, 0)
      setInstances(res?.data || [])
    } catch {
      setInstances([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
  }, [])

  const filtered = clusterFilter
    ? instances.filter((i) => i.cluster_id === clusterFilter)
    : instances

  const clusters: Record<string, ClusterTopology> = {}
  const standalones: Instance[] = []
  for (const inst of filtered) {
    if (!inst.cluster_id) {
      standalones.push(inst)
      continue
    }
    if (!clusters[inst.cluster_id]) {
      clusters[inst.cluster_id] = { cluster_id: inst.cluster_id, master: null, replicas: [] }
    }
    const role = inst.status?.role || ''
    if (role === 'master' || role === 'primary' || role === 'primary_master') {
      clusters[inst.cluster_id].master = inst
    } else {
      clusters[inst.cluster_id].replicas.push(inst)
    }
  }

  const clusterIds = Array.from(new Set(instances.map((i) => i.cluster_id).filter(Boolean)))

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title={
          <Space>
            <ApartmentOutlined />
            <span>集群拓扑</span>
          </Space>
        }
        extra={
          <Space>
            <Select
              placeholder="按集群过滤"
              allowClear
              style={{ width: 220 }}
              value={clusterFilter}
              onChange={setClusterFilter}
              options={clusterIds.map((c) => ({ value: c as string, label: c as string }))}
            />
            <Button icon={<ReloadOutlined />} onClick={fetchData}>刷新</Button>
          </Space>
        }
      >
        <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
          <Col span={6}>
            <Card>
              <Statistic title="实例总数" value={filtered.length} prefix={<DatabaseOutlined />} />
            </Card>
          </Col>
          <Col span={6}>
            <Card>
              <Statistic
                title="集群数"
                value={Object.keys(clusters).length}
                prefix={<ApartmentOutlined />}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card>
              <Statistic
                title="主节点数"
                value={Object.values(clusters).filter((c) => c.master).length}
                prefix={<DatabaseOutlined />}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card>
              <Statistic
                title="独立实例"
                value={standalones.length}
                prefix={<DatabaseOutlined />}
              />
            </Card>
          </Col>
        </Row>

        {loading ? (
          <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
        ) : filtered.length === 0 ? (
          <Empty description="暂无实例数据, 请先创建实例" />
        ) : (
          <>
            <Row gutter={[16, 16]}>
              {Object.values(clusters).map((cluster) => (
                <Col xs={24} md={12} lg={8} key={cluster.cluster_id}>
                  <Card
                    size="small"
                    title={
                      <Space>
                        <ApartmentOutlined />
                        <span>集群: {cluster.cluster_id}</span>
                      </Space>
                    }
                  >
                    <div style={{ marginBottom: 8 }}>
                      <strong>主节点:</strong>{' '}
                      {cluster.master ? (
                        <Tag color="blue">{cluster.master.name}</Tag>
                      ) : (
                        <Tag color="warning">无主</Tag>
                      )}
                    </div>
                    <div>
                      <strong>从节点 ({cluster.replicas.length}):</strong>
                      <div style={{ marginTop: 4 }}>
                        {cluster.replicas.length === 0 ? (
                          <Tag>无</Tag>
                        ) : (
                          cluster.replicas.map((r) => (
                            <Tag key={r.id} color="default">{r.name}</Tag>
                          ))
                        )}
                      </div>
                    </div>
                  </Card>
                </Col>
              ))}
            </Row>
            {standalones.length > 0 && (
              <div style={{ marginTop: 16 }}>
                <div style={{ marginBottom: 8, fontWeight: 600 }}>独立实例 (未加入集群)</div>
                <Row gutter={[16, 16]}>
                  {standalones.map((s) => (
                    <Col xs={24} md={12} lg={8} key={s.id}>
                      <Card size="small" title={<Space><DatabaseOutlined /><span>{s.name}</span></Space>}>
                        <div>主机: {s.host || s.connection?.host || '-'}</div>
                        <div>端口: {s.port || s.connection?.port || '-'}</div>
                        <div>角色: {s.status?.role ? <Tag color="default">{s.status.role}</Tag> : <Tag>未检测</Tag>}</div>
                      </Card>
                    </Col>
                  ))}
                </Row>
              </div>
            )}
          </>
        )}
      </Card>
    </div>
  )
}

export default TopologyView
