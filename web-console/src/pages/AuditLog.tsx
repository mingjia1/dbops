import React, { useEffect, useState } from 'react'
import { Card, Table, Tag, Input, Space, Button } from 'antd'
import { SearchOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { auditApi, type AuditLog as AuditLogType } from '../services/api'

const AuditLog: React.FC = () => {
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [filters, setFilters] = useState<{ user?: string; action?: string }>({})

  const fetchData = (f?: any) => {
    const f2 = f || filters
    setLoading(true)
    auditApi.list(f2).then((res: any) => {
      setData(res?.data || [])
    }).catch(() => {}).finally(() => setLoading(false))
  }

  useEffect(() => { fetchData() }, [])

  // P1: 之前 Input onChange 立即 setFilters, 配合 handleSearch 按钮 trigger
  // 重复 fetch, 且搜索按钮连点会发重复请求. 修: 加 300ms debounce
  // (输入稳定后再 fetch), 搜索按钮变手动 retry.
  useEffect(() => {
    const timer = setTimeout(() => { fetchData() }, 300)
    return () => clearTimeout(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filters])

  const handleSearch = () => fetchData()

  const columns: ColumnsType<AuditLogType> = [
    { title: '用户', dataIndex: 'user', key: 'user', render: (u) => u || '-' },
    { title: '操作', dataIndex: 'action', key: 'action' },
    { title: '资源', dataIndex: 'resource_type', key: 'resource_type', render: (r) => r || '-' },
    {
      title: '结果',
      dataIndex: 'result',
      key: 'result',
      render: (result) => (
        <Tag color={result === 'success' ? 'success' : 'error'}>
          {result === 'success' ? '成功' : '失败'}
        </Tag>
      ),
    },
    { title: 'IP地址', dataIndex: 'ip_address', key: 'ip_address' },
    {
      title: '时间',
      dataIndex: 'created_at',
      key: 'created_at',
      // P1: 之前直接展示 ISO 字符串 "2024-12-04T10:30:00Z" 不可读.
      // 修: toLocaleString 本地化显示.
      render: (t: string) => (t ? new Date(t).toLocaleString() : '-'),
    },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title="审计日志"
        extra={
          <Space>
            <Input
              placeholder="搜索用户"
              style={{ width: 140 }}
              prefix={<SearchOutlined />}
              allowClear
              value={filters.user}
              onChange={(e) => setFilters({ ...filters, user: e.target.value })}
            />
            <Input
              placeholder="操作类型"
              style={{ width: 140 }}
              allowClear
              value={filters.action}
              onChange={(e) => setFilters({ ...filters, action: e.target.value })}
            />
            <Button type="primary" icon={<SearchOutlined />} onClick={handleSearch}>搜索</Button>
            <Button icon={<ReloadOutlined />} onClick={() => { setFilters({}); fetchData({}) }}>重置</Button>
          </Space>
        }
      >
        <Table columns={columns} dataSource={data} rowKey="id" loading={loading} />
      </Card>
    </div>
  )
}

export default AuditLog