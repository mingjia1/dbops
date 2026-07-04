import React from 'react'
import { Button, Empty, Progress, Select, Space, Table, Tag } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, DeleteOutlined, EyeOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import {
  type DeployResult, type ArchType,
  normalizeStatus, getStatusCategory,
  isCompletedDeployStatus, isDestroyedDeployStatus, isPartialDeployStatus, isFailedDeployStatus,
  deploymentProgress, deploymentProgressStatus,
} from '../services/deployHelpers'
import { formatClusterRole } from '../services/roleDisplay'
import type { Instance } from '../services/api'

interface DeploymentHistoryPanelProps {
  loading: boolean
  dataSource: DeployResult[]
  statusFilter: string[]
  archFilter: ArchType | 'all'
  instances: Instance[]
  onStatusFilterChange: (values: string[]) => void
  onArchFilterChange: (value: ArchType | 'all') => void
  onViewPlan: (record: DeployResult) => void
  onDestroy: (record: DeployResult) => void
}

const deploymentNodes = (record: DeployResult, instances: Instance[]): string[] => {
  if (record.nodes?.length) {
    return record.nodes.map((node) => {
      const endpoint = `${node.host || '-'}:${node.port || '-'}`
      return `${node.name || node.instance_id || '-'} (${endpoint}, ${formatClusterRole(record.cluster_type, node.role)})`
    })
  }
  const clusterID = record.cluster_id || record.deployment_id
  return instances
    .filter((inst) => inst.cluster_id === clusterID)
    .map((inst) => {
      const endpoint = `${inst.connection?.host || inst.host || '-'}:${inst.connection?.port || inst.port || '-'}`
      const role = formatClusterRole(record.cluster_type, inst.status?.role)
      return `${inst.name} (${endpoint}, ${role})`
    })
}

const DeploymentHistoryPanel: React.FC<DeploymentHistoryPanelProps> = ({
  loading, dataSource, statusFilter, archFilter, instances,
  onStatusFilterChange, onArchFilterChange, onViewPlan, onDestroy,
}) => {
  const columns: ColumnsType<DeployResult> = [
    { title: '部署ID', dataIndex: 'deployment_id', key: 'deployment_id', width: 180, ellipsis: true },
    { title: '集群ID', dataIndex: 'cluster_id', key: 'cluster_id', width: 150, ellipsis: true },
    {
      title: '架构',
      dataIndex: 'cluster_type',
      key: 'cluster_type',
      width: 80,
      render: (type: ArchType) => <Tag color={type === 'ha' ? 'cyan' : type === 'mha' ? 'blue' : type === 'mgr' ? 'green' : 'orange'}>{type.toUpperCase()}</Tag>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 110,
      render: (status: string) => {
        if (isCompletedDeployStatus(status)) return <Tag color="success" icon={<CheckCircleOutlined />}>成功</Tag>
        if (isDestroyedDeployStatus(status)) return <Tag color="default">已销毁</Tag>
        if (isPartialDeployStatus(status)) return <Tag color="warning" icon={<CloseCircleOutlined />}>部分完成</Tag>
        if (isFailedDeployStatus(status)) return <Tag color="error" icon={<CloseCircleOutlined />}>失败</Tag>
        if (normalizeStatus(status) === 'pending') return <Tag color="default">待开始</Tag>
        return <Tag color="processing" icon={<ReloadOutlined spin />}>进行中</Tag>
      },
    },
    { title: '当前阶段', dataIndex: 'stage', key: 'stage', width: 100, ellipsis: true, render: (stage: string) => stage || '-' },
    {
      title: '进度',
      dataIndex: 'progress',
      key: 'progress',
      width: 140,
      render: (progress: number, record) => <Progress percent={deploymentProgress(record.status, progress)} size="small" status={deploymentProgressStatus(record.status)} />,
    },
    { title: '信息', dataIndex: 'message', key: 'message', ellipsis: true },
    {
      title: '节点信息',
      key: 'nodes',
      width: 200,
      ellipsis: true,
      render: (_, record) => {
        const nodes = deploymentNodes(record, instances)
        if (nodes.length === 0) return '-'
        return (
          <Space direction="vertical" size={2}>
            {nodes.map((node) => <span key={node}>{node}</span>)}
          </Space>
        )
      },
    },
    { title: '开始时间', dataIndex: 'started_at', key: 'started_at', width: 160, render: (time: string) => (time ? new Date(time).toLocaleString() : '-') },
    {
      title: '操作',
      key: 'action',
      width: 140,
      render: (_, record) => (
        <Space>
          <Button size="small" icon={<EyeOutlined />} onClick={() => onViewPlan(record)}>
            查看计划
          </Button>
          <Button size="small" danger icon={<DeleteOutlined />} disabled={isDestroyedDeployStatus(record.status)} onClick={() => onDestroy(record)}>
            销毁
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Select
          mode="multiple"
          placeholder="筛选状态"
          value={statusFilter}
          onChange={onStatusFilterChange}
          style={{ minWidth: 200 }}
          maxTagCount="responsive"
          options={[
            { label: '成功', value: 'success' },
            { label: '失败', value: 'failed' },
            { label: '部分完成', value: 'partial' },
            { label: '运行中', value: 'running' },
            { label: '待开始', value: 'pending' },
            { label: '已销毁', value: 'destroyed' },
          ]}
        />
        <Select
          placeholder="筛选架构"
          value={archFilter}
          onChange={onArchFilterChange}
          style={{ width: 120 }}
          options={[
            { label: '全部架构', value: 'all' },
            { label: 'HA', value: 'ha' },
            { label: 'MHA', value: 'mha' },
            { label: 'MGR', value: 'mgr' },
            { label: 'PXC', value: 'pxc' },
          ]}
        />
      </Space>
      {dataSource.length === 0 ? (
        <Empty description="暂无符合条件的部署记录" />
      ) : (
        <Table
          columns={columns}
          dataSource={dataSource}
          rowKey="deployment_id"
          loading={loading}
          scroll={{ x: 'max-content' }}
          pagination={{
            defaultPageSize: 10,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (t) => `共 ${t} 条记录`,
          }}
        />
      )}
    </div>
  )
}

export default DeploymentHistoryPanel
