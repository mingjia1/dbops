import React, { useState, useEffect } from 'react'
import { Card, Tabs, Button, Space, Tag, Modal, Descriptions, message, Empty, Spin } from 'antd'
import { ClusterOutlined, DatabaseOutlined, ReloadOutlined } from '@ant-design/icons'
import { useDispatch, useSelector } from 'react-redux'
import { fetchInstances } from '@/store/instanceSlice'
import type { RootState } from '@/store'

interface InstanceNode {
  id: string
  name: string
  host: string
  port: number
  role: 'master' | 'slave' | 'single'
  cluster_id?: string
  status: 'online' | 'offline'
}

interface ClusterTopology {
  cluster_id: string
  master: InstanceNode
  slaves: InstanceNode[]
}

const InstanceTopology: React.FC = () => {
  const dispatch = useDispatch()
  const { instances, loading } = useSelector((state: RootState) => state.instances as any)
  const [selectedNode, setSelectedNode] = useState<InstanceNode | null>(null)

  useEffect(() => {
    dispatch(fetchInstances() as any)
  }, [dispatch])

  const handleNodeClick = (node: InstanceNode) => {
    setSelectedNode(node)
    message.info(`选中实例: ${node.name}`)
  }

  const instanceNodes: InstanceNode[] = instances?.map((inst: any) => ({
    id: inst.id,
    name: inst.name,
    host: inst.host,
    port: inst.port,
    role: inst.role || 'single',
    cluster_id: inst.cluster_id,
    status: 'online' as const,
  })) || []

  return (
    <div style={{ padding: '20px' }}>
      <div style={{ marginBottom: 16 }}>
        <Button icon={<ReloadOutlined />} onClick={() => dispatch(fetchInstances() as any)}>
          刷新拓扑
        </Button>
      </div>

      {loading ? (
        <Spin tip="加载中..." />
      ) : instanceNodes.length === 0 ? (
        <Empty description="暂无实例数据" />
      ) : (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '16px' }}>
          {instanceNodes.map((node) => (
            <Card
              key={node.id}
              hoverable
              style={{ width: 240, cursor: 'pointer' }}
              onClick={() => handleNodeClick(node)}
            >
              <div style={{ textAlign: 'center' }}>
                <DatabaseOutlined style={{ fontSize: 32, marginBottom: 8 }} />
                <h3>{node.name}</h3>
                <Tag color={node.status === 'online' ? 'green' : 'red'}>
                  {node.status === 'online' ? '在线' : '离线'}
                </Tag>
                <Tag color={node.role === 'master' ? 'blue' : node.role === 'slave' ? 'orange' : 'default'}>
                  {node.role === 'master' ? '主节点' : node.role === 'slave' ? '从节点' : '单点'}
                </Tag>
                <p style={{ marginTop: 8, color: '#666' }}>{node.host}:{node.port}</p>
              </div>
            </Card>
          ))}
        </div>
      )}

      <Modal
        title="实例详情"
        open={!!selectedNode}
        onCancel={() => setSelectedNode(null)}
        footer={null}
      >
        {selectedNode && (
          <Descriptions column={1}>
            <Descriptions.Item label="ID">{selectedNode.id}</Descriptions.Item>
            <Descriptions.Item label="名称">{selectedNode.name}</Descriptions.Item>
            <Descriptions.Item label="地址">{selectedNode.host}:{selectedNode.port}</Descriptions.Item>
            <Descriptions.Item label="角色">{selectedNode.role}</Descriptions.Item>
            <Descriptions.Item label="状态">{selectedNode.status}</Descriptions.Item>
            {selectedNode.cluster_id && (
              <Descriptions.Item label="集群ID">{selectedNode.cluster_id}</Descriptions.Item>
            )}
          </Descriptions>
        )}
      </Modal>
    </div>
  )
}

const ClusterTopology: React.FC = () => {
  const { instances } = useSelector((state: RootState) => state.instances as any)
  const [selectedCluster, setSelectedCluster] = useState<ClusterTopology | null>(null)

  const clusterMap = new Map<string, InstanceNode[]>()
  
  instances?.forEach((inst: any) => {
    if (inst.cluster_id) {
      if (!clusterMap.has(inst.cluster_id)) {
        clusterMap.set(inst.cluster_id, [])
      }
      const node: InstanceNode = {
        id: inst.id,
        name: inst.name,
        host: inst.host,
        port: inst.port,
        role: inst.role || 'single',
        cluster_id: inst.cluster_id,
        status: 'online',
      }
      clusterMap.get(inst.cluster_id)!.push(node)
    }
  })

  const clusters: ClusterTopology[] = []
  clusterMap.forEach((nodes, clusterId) => {
    const master = nodes.find(n => n.role === 'master') || nodes[0]
    const slaves = nodes.filter(n => n.role === 'slave')
    clusters.push({ cluster_id: clusterId, master, slaves })
  })

  const handleClusterClick = (cluster: ClusterTopology) => {
    setSelectedCluster(cluster)
    message.info(`选中集群: ${cluster.cluster_id}`)
  }

  return (
    <div style={{ padding: '20px' }}>
      {clusters.length === 0 ? (
        <Empty description="暂无集群数据" />
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
          {clusters.map((cluster) => (
            <Card
              key={cluster.cluster_id}
              title={
                <Space>
                  <ClusterOutlined />
                  <span>集群: {cluster.cluster_id}</span>
                </Space>
              }
              hoverable
              style={{ cursor: 'pointer' }}
              onClick={() => handleClusterClick(cluster)}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '24px' }}>
                <Card
                  size="small"
                  style={{ borderColor: '#1890ff' }}
                  bodyStyle={{ textAlign: 'center' }}
                >
                  <DatabaseOutlined style={{ fontSize: 24, color: '#1890ff' }} />
                  <div style={{ marginTop: 8, fontWeight: 'bold' }}>主节点</div>
                  <div>{cluster.master.name}</div>
                  <div style={{ fontSize: 12, color: '#666' }}>
                    {cluster.master.host}:{cluster.master.port}
                  </div>
                </Card>

                {cluster.slaves.length > 0 && (
                  <>
                    <div style={{ width: 60, height: 2, backgroundColor: '#d9d9d9' }} />
                    <div style={{ display: 'flex', gap: '16px' }}>
                      {cluster.slaves.map((slave) => (
                        <Card
                          key={slave.id}
                          size="small"
                          style={{ borderColor: '#faad14' }}
                          bodyStyle={{ textAlign: 'center' }}
                        >
                          <DatabaseOutlined style={{ fontSize: 24, color: '#faad14' }} />
                          <div style={{ marginTop: 8 }}>从节点</div>
                          <div>{slave.name}</div>
                          <div style={{ fontSize: 12, color: '#666' }}>
                            {slave.host}:{slave.port}
                          </div>
                        </Card>
                      ))}
                    </div>
                  </>
                )}
              </div>
            </Card>
          ))}
        </div>
      )}

      <Modal
        title={`集群详情: ${selectedCluster?.cluster_id}`}
        open={!!selectedCluster}
        onCancel={() => setSelectedCluster(null)}
        footer={null}
      >
        {selectedCluster && (
          <div>
            <h4>主节点</h4>
            <Descriptions column={1} size="small">
              <Descriptions.Item label="名称">{selectedCluster.master.name}</Descriptions.Item>
              <Descriptions.Item label="地址">
                {selectedCluster.master.host}:{selectedCluster.master.port}
              </Descriptions.Item>
            </Descriptions>
            
            {selectedCluster.slaves.length > 0 && (
              <>
                <h4 style={{ marginTop: 16 }}>从节点 ({selectedCluster.slaves.length}个)</h4>
                {selectedCluster.slaves.map((slave, idx) => (
                  <Descriptions key={slave.id} column={1} size="small" title={`从节点 ${idx + 1}`}>
                    <Descriptions.Item label="名称">{slave.name}</Descriptions.Item>
                    <Descriptions.Item label="地址">{slave.host}:{slave.port}</Descriptions.Item>
                  </Descriptions>
                ))}
              </>
            )}
          </div>
        )}
      </Modal>
    </div>
  )
}

const TopologyView: React.FC = () => {
  const items = [
    {
      key: 'instance',
      label: '实例拓扑',
      children: <InstanceTopology />,
    },
    {
      key: 'cluster',
      label: '集群拓扑',
      children: <ClusterTopology />,
    },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <h2 style={{ marginBottom: 24 }}>
        <DatabaseOutlined style={{ marginRight: 8 }} />
        MySQL 实例拓扑图
      </h2>
      <Tabs items={items} />
    </div>
  )
}

export default TopologyView