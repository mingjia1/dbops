import React from 'react'
import { Button, Space, Table, Tag } from 'antd'

interface PrecheckResultItem {
  host_id: string
  host: string
  status: string
  message?: string
  details?: Array<{
    name: string
    passed: boolean
    value?: string
    message?: string
    fixable?: boolean
    fix_action?: string
    payload?: Record<string, string>
    port?: number
  }>
}

interface PrecheckResultTableProps {
  results: PrecheckResultItem[]
  loading: boolean
  onRepair: (record: PrecheckResultItem, detail: PrecheckResultItem['details'][0]) => void
}

const PrecheckResultTable: React.FC<PrecheckResultTableProps> = ({ results, loading, onRepair }) => {
  return (
    <div style={{ marginTop: 16 }}>
      <strong>环境预检结果</strong>
      <Table
        size="small"
        pagination={false}
        style={{ marginTop: 8 }}
        dataSource={results}
        rowKey="host_id"
        columns={[
          { title: '主机', dataIndex: 'host', key: 'host' },
          {
            title: '状态',
            dataIndex: 'status',
            key: 'status',
            render: (s: string) => (
              <Tag color={s === 'pass' ? 'success' : s === 'warn' ? 'warning' : 'error'}>
                {s === 'pass' ? '通过' : s === 'warn' ? '警告' : '失败'}
              </Tag>
            ),
          },
          { title: '消息', dataIndex: 'message', key: 'message' },
          {
            title: '详情',
            dataIndex: 'details',
            key: 'details',
            render: (details: PrecheckResultItem['details'], record: PrecheckResultItem) => (
              <Space direction="vertical" size={2}>
                {(details || []).map((d, i) => (
                  <span key={i} style={{ fontSize: 12 }}>
                    <Tag color={d.passed ? 'success' : 'error'} style={{ fontSize: 10 }}>{d.name}</Tag>
                    {d.value && <span style={{ color: '#888' }}>{d.value} </span>}
                    {d.message && <span style={{ color: '#ff4d4f' }}>{d.message}</span>}
                    {!d.passed && d.fixable && (
                      <Button
                        size="small"
                        type="link"
                        loading={loading}
                        onClick={() => onRepair(record, d)}
                        style={{ padding: '0 4px', height: 20 }}
                      >
                        修复
                      </Button>
                    )}
                  </span>
                ))}
              </Space>
            ),
          },
        ]}
      />
    </div>
  )
}

export default PrecheckResultTable
