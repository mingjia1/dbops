import React, { useMemo } from 'react'
import { Empty } from 'antd'
import { Space, Tag, Typography } from 'antd'
import {
  Background,
  Controls,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
  Handle,
  Position,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
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

const NODE_W = 190
const NODE_H = 92
const GAP_X = 110
const GAP_Y = 36

const FlowNodeCard: React.FC<NodeProps> = ({ data }) => {
  const d = data as {
    label: string
    endpoint: string
    role: string
    status: string
    arch?: string
  }
  return (
    <div style={{
      width: NODE_W,
      border: '1px solid var(--apple-separator)',
      borderRadius: 8,
      padding: 10,
      background: 'var(--apple-bg-elevated)',
      boxShadow: 'var(--apple-shadow-1)',
    }}>
      <Handle type="target" position={Position.Left} style={{ background: 'var(--apple-text-tertiary)' }} />
      <Space direction="vertical" size={4} style={{ width: '100%' }}>
        <Text strong ellipsis title={d.label}>{d.label}</Text>
        <Text type="secondary" style={{ fontSize: 12 }}>{d.endpoint}</Text>
        <Space size={4} wrap>
          <Tag color={roleColor(d.role)} style={{ marginRight: 0 }}>{formatClusterRole(d.arch, d.role) || 'unknown'}</Tag>
          <Tag color={statusColor(d.status)} style={{ marginRight: 0 }}>{d.status || 'unknown'}</Tag>
        </Space>
      </Space>
      <Handle type="source" position={Position.Right} style={{ background: 'var(--apple-text-tertiary)' }} />
    </div>
  )
}

const nodeTypes = { topology: FlowNodeCard }

export const TopologyGraph: React.FC<TopologyGraphProps> = ({ nodes, edges, instanceByID, arch }) => {
  // 链式分层布局: 入度 0 的节点放第一列, 沿复制关系向右逐列展开, 每列纵向堆叠.
  const { flowNodes, flowEdges } = useMemo(() => {
    const depth = new Map<string, number>()
    const byId = new Map(nodes.map((node) => [node.id, node]))
    const roots = nodes.filter((node) => !edges.some((edge) => edge.target_id === node.id))
    const queue = (roots.length > 0 ? roots : nodes.slice(0, 1)).map((node) => node.id)
    queue.forEach((id) => depth.set(id, 0))
    let visited = new Set<string>(queue)
    let current = [...queue]
    let maxDepth = 0
    while (current.length > 0) {
      const next: string[] = []
      for (const id of current) {
        const d = depth.get(id) ?? 0
        maxDepth = Math.max(maxDepth, d)
        for (const edge of edges.filter((e) => e.source_id === id)) {
          const tid = edge.target_id
          if (!byId.has(tid)) continue
          const nd = depth.has(tid) ? Math.max(depth.get(tid)!, d + 1) : d + 1
          depth.set(tid, nd)
          maxDepth = Math.max(maxDepth, nd)
          if (!visited.has(tid)) {
            visited.add(tid)
            next.push(tid)
          }
        }
      }
      current = next
    }
    // 孤立节点(无任何边引用)兜底放到最后一列
    for (const node of nodes) {
      if (!depth.has(node.id)) depth.set(node.id, maxDepth)
    }

    const columnCount: Record<number, number> = {}
    const flowNodes: Node[] = nodes.map((node) => {
      const d = depth.get(node.id) ?? 0
      const row = columnCount[d] ?? 0
      columnCount[d] = row + 1
      const instance = instanceByID.get(node.id)
      return {
        id: node.id,
        type: 'topology',
        position: { x: d * (NODE_W + GAP_X), y: row * (NODE_H + GAP_Y) },
        data: {
          label: node.name || node.id,
          endpoint: endpointOf(instance),
          role: node.role,
          status: node.status,
          arch,
        },
      }
    })

    const flowEdges: Edge[] = edges
      .filter((edge) => byId.has(edge.source_id) && byId.has(edge.target_id))
      .map((edge, index) => ({
        id: `e-${edge.source_id}-${edge.target_id}-${index}`,
        source: edge.source_id,
        target: edge.target_id,
        label: relationLabel(edge.label || edge.type),
        animated: true,
        style: { stroke: 'var(--apple-text-secondary)' },
        labelStyle: { fontSize: 12, fill: 'var(--apple-text-secondary)' },
        labelBgStyle: { fill: 'var(--apple-bg-elevated)', fillOpacity: 0.85 },
      }))

    return { flowNodes, flowEdges }
  }, [nodes, edges, instanceByID, arch])

  if (nodes.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无节点" style={{ margin: '12px 0' }} />
  }

  const width = Math.min(
    1100,
    Math.max(
      480,
      (new Set(nodes.map((n) => depthOf(nodes, edges, n.id))).size || 1) * (NODE_W + GAP_X) + 40,
    ),
  )
  const height = Math.max(180, Math.min(560, (largestColumn(nodes, edges) || 1) * (NODE_H + GAP_Y) + 40))

  return (
    <div style={{ width: '100%', height, border: '1px solid var(--apple-separator)', borderRadius: 8, marginBottom: 16 }}>
      <ReactFlow
        nodes={flowNodes}
        edges={flowEdges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.15 }}
        nodesConnectable={false}
        elementsSelectable={false}
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={18} />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  )
}

// 求单个节点的层级深度 (用于容器尺寸估算, 与 useMemo 内布局同规则).
function depthOf(nodes: TopologyNode[], edges: TopologyEdge[], id: string): number {
  const incoming = edges.filter((edge) => edge.target_id === id)
  if (incoming.length === 0) return 0
  return Math.max(...incoming.map((edge) => depthOf(nodes, edges, edge.source_id) + 1))
}

function largestColumn(nodes: TopologyNode[], edges: TopologyEdge[]): number {
  const counts: Record<number, number> = {}
  for (const node of nodes) {
    const d = depthOf(nodes, edges, node.id)
    counts[d] = (counts[d] || 0) + 1
  }
  return Math.max(...Object.values(counts), 1)
}
