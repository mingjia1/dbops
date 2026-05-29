import React, { useEffect, useState } from 'react'
import { Table, Button, Space, DatePicker, Select, Input, message } from 'antd'
import { SearchOutlined, ReloadOutlined } from '@ant-design/icons'
import { auditApi } from '@/services/api'
import type { AuditLog as AuditLogType } from '@/services/api'

const { RangePicker } = DatePicker

const AuditLog: React.FC = () => {
  const [logs, setLogs] = useState<AuditLogType[]>([])
  const [loading, setLoading] = useState(false)
  const [filters, setFilters] = useState({
    user: '',
    action: '',
    start_date: '',
    end_date: '',
  })

  useEffect(() => {
    loadLogs()
  }, [])

  const loadLogs = async () => {
    setLoading(true)
    try {
      const data = await auditApi.list(filters) as any
      setLogs(data)
    } catch (err) {
      message.error('加载审计日志失败')
    } finally {
      setLoading(false)
    }
  }

  const handleSearch = () => {
    loadLogs()
  }

  const handleReset = () => {
    setFilters({
      user: '',
      action: '',
      start_date: '',
      end_date: '',
    })
    loadLogs()
  }

  const handleDateChange = (dates: any) => {
    if (dates) {
      setFilters({
        ...filters,
        start_date: dates[0].format('YYYY-MM-DD'),
        end_date: dates[1].format('YYYY-MM-DD'),
      })
    } else {
      setFilters({
        ...filters,
        start_date: '',
        end_date: '',
      })
    }
  }

  const getActionTag = (action: string) => {
    const actionMap: Record<string, string> = {
      create: 'green',
      update: 'blue',
      delete: 'red',
      login: 'orange',
      logout: 'default',
    }
    return actionMap[action] || 'default'
  }

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 150,
    },
    {
      title: '用户',
      dataIndex: 'user',
      key: 'user',
    },
    {
      title: '操作',
      dataIndex: 'action',
      key: 'action',
      render: (text: string) => (
        <span style={{ color: getActionTag(text) }}>{text}</span>
      ),
    },
    {
      title: '资源类型',
      dataIndex: 'resource_type',
      key: 'resource_type',
    },
    {
      title: '资源ID',
      dataIndex: 'resource_id',
      key: 'resource_id',
    },
    {
      title: 'IP地址',
      dataIndex: 'ip_address',
      key: 'ip_address',
    },
    {
      title: '详情',
      dataIndex: 'details',
      key: 'details',
      ellipsis: true,
    },
    {
      title: '时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
    },
  ]

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Space size="middle" wrap>
          <Input
            placeholder="用户名"
            value={filters.user}
            onChange={(e) => setFilters({ ...filters, user: e.target.value })}
            style={{ width: 150 }}
          />
          <Select
            placeholder="操作类型"
            value={filters.action || undefined}
            onChange={(value) => setFilters({ ...filters, action: value || '' })}
            style={{ width: 150 }}
            allowClear
          >
            <Select.Option value="create">创建</Select.Option>
            <Select.Option value="update">更新</Select.Option>
            <Select.Option value="delete">删除</Select.Option>
            <Select.Option value="login">登录</Select.Option>
            <Select.Option value="logout">登出</Select.Option>
          </Select>
          <RangePicker onChange={handleDateChange} />
          <Button type="primary" icon={<SearchOutlined />} onClick={handleSearch}>
            查询
          </Button>
          <Button icon={<ReloadOutlined />} onClick={handleReset}>
            重置
          </Button>
        </Space>
      </div>

      <Table
        columns={columns}
        dataSource={logs}
        rowKey="id"
        loading={loading}
        scroll={{ x: 1200 }}
      />
    </div>
  )
}

export default AuditLog