import React, { useMemo } from 'react'
import { Card, Table, Tag, Space } from 'antd'
import { DatabaseOutlined, CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { CAPABILITY_LABELS, hasCapability, type Capability, TIERED_ONBOARDING_FLAVORS } from '../services/flavorCapability'

interface FlavorRow {
  key: string
  flavor: string
  capabilities: Record<Capability, boolean>
}

const ALL_CAPABILITIES: Capability[] = [
  'replication',
  'failover',
  'cluster_deploy',
  'backup_physical',
  'upgrade_inplace',
  'health_sql',
  'scale',
  'node_rebuild',
  'instance_deploy',
  'upgrade_logical',
  'parameter_template',
  'instance_admin',
]

const MYSQL_PROTOCOL_FLAVORS = ['mysql', 'mariadb', 'percona']

export default function CapabilityMatrix() {
  const dataSource = useMemo<FlavorRow[]>(() => {
    const rows: FlavorRow[] = []
    for (const flavor of MYSQL_PROTOCOL_FLAVORS) {
      const capabilities: Record<Capability, boolean> = {} as Record<Capability, boolean>
      for (const cap of ALL_CAPABILITIES) {
        capabilities[cap] = hasCapability(flavor, cap)
      }
      rows.push({ key: flavor, flavor, capabilities })
    }
    for (const flavor of Object.keys(TIERED_ONBOARDING_FLAVORS)) {
      const capabilities: Record<Capability, boolean> = {} as Record<Capability, boolean>
      for (const cap of ALL_CAPABILITIES) {
        capabilities[cap] = hasCapability(flavor, cap)
      }
      rows.push({ key: flavor, flavor, capabilities })
    }
    return rows
  }, [])

  const columns: ColumnsType<FlavorRow> = [
    {
      title: '数据库引擎',
      dataIndex: 'flavor',
      key: 'flavor',
      render: (text: string) => (
        <Space><DatabaseOutlined />{text.toUpperCase()}</Space>
      ),
    },
    ...ALL_CAPABILITIES.map((cap) => ({
      title: CAPABILITY_LABELS[cap],
      key: cap,
      align: 'center' as const,
      render: (_: unknown, record: FlavorRow) => {
        const supported = record.capabilities[cap]
        return supported
          ? <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 18 }} />
          : <CloseCircleOutlined style={{ color: '#d9d9d9', fontSize: 18 }} />
      },
    })),
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={<><DatabaseOutlined /> 引擎能力矩阵</>}
        extra={<Tag color="blue">只读展示</Tag>}
      >
        <Table
          dataSource={dataSource}
          columns={columns}
          pagination={false}
          bordered
          size="middle"
        />
      </Card>
    </div>
  )
}
