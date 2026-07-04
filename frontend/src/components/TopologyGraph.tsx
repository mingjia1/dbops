import React from 'react'
import { Empty, Space, Tag, Typography } from 'antd'
import { formatClusterRole } from '../services/roleDisplay'
import {
  TopologyNode, TopologyEdge,
  roleColor, statusColor, endpointOf, relationLabel,
  primaryRoles, replicaRoles,
} from '../services/topologyHelpers'
import type { Instance } from '../services/api'

const { Text } = Typography

interface TopologyGraphProps {
  nodes: TopologyNode[]
  edges: TopologyEdge[]
  instanceByID: Map<string, Instance>
  arch?: string
}

const GraphNode: React.FC<{
  node: TopologyNode
  instance?: Instance
  arch?: string
}> = ({ node, instance, arch }) => (
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

export const TopologyGraph: React.FC<TopologyGraphProps> = ({ nodes, edges, instanceByID, arch }) => {
  if (nodes.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无节点" style={{ margin: '12px 0' }} />
  }

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
    return { root, chain, rowEdges: edges.filter((edge) => edge.source_id === root.id) }
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
                  <Tag>{relationLabel(row.rowEdges[index - 1]?.label || row.rowEdges[index - 1]?.type)}</Tag>
                  <span style={{ fontSize: 18, lineHeight: 1 }}>→</span>
                </div>
              )}
              <GraphNode node={node} instance={instanceByID.get(node.id)} arch={arch} />
            </React.Fragment>
          ))}
        </div>
      ))}
      {detached.length > 0 && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 'max-content' }}>
          {detached.map((node) => (
            <GraphNode key={node.id} node={node} instance={instanceByID.get(node.id)} arch={arch} />
          ))}
        </div>
      )}
    </Space>
  )
}
