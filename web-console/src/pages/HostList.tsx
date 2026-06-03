import React, { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, Table, Button, Space, Tag, Popconfirm, message } from 'antd'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { hostApi, Host } from '../services/api'

const HostList: React.FC = () => {
  const navigate = useNavigate()
  const [hosts, setHosts] = useState<Host[]>([])
  const [loading, setLoading] = useState(false)

  const fetchHosts = async () => {
    setLoading(true)
    try {
      const res: any = await hostApi.list()
      setHosts(res.data || [])
    } catch {
      setHosts([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchHosts()
  }, [])

  const handleDelete = async (id: string) => {
    try {
      await hostApi.delete(id)
      message.success('主机删除成功')
      fetchHosts()
    } catch {
      // interceptor already showed error
    }
  }

  const columns: ColumnsType<Host> = [
    { title: '主机名称', dataIndex: 'name', key: 'name' },
    {
      title: '地址',
      key: 'address',
      render: (_, r) => `${r.address}:${r.ssh_port}`,
    },
    { title: 'SSH 用户', dataIndex: 'ssh_user', key: 'ssh_user' },
    {
      title: '认证方式',
      dataIndex: 'ssh_auth_method',
      key: 'ssh_auth_method',
      render: (m) => <Tag>{m === 'password' ? '密码' : '密钥'}</Tag>,
    },
    {
      title: '操作系统',
      dataIndex: 'os_type',
      key: 'os_type',
      render: (os) => os?.toUpperCase() || '-',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status) => {
        const colorMap: Record<string, string> = {
          success: 'success',
          failed: 'error',
          unknown: 'default',
          pending: 'processing',
        }
        const textMap: Record<string, string> = {
          success: '可用',
          failed: '不可用',
          unknown: '未检测',
          pending: '检测中',
        }
        return <Tag color={colorMap[status] || 'default'}>{textMap[status] || status}</Tag>
      },
    },
    {
      title: '最后检测',
      dataIndex: 'last_check_at',
      key: 'last_check_at',
      render: (t) => (t ? new Date(t).toLocaleString() : '-'),
    },
    {
      title: '操作',
      key: 'action',
      render: (_, r) => (
        <Space>
          <Button type="link" size="small" onClick={() => navigate(`/dashboard/hosts/${r.id}`)}>
            详情
          </Button>
          <Button type="link" size="small" onClick={() => navigate(`/dashboard/hosts/${r.id}/edit`)}>
            编辑
          </Button>
          <Popconfirm
            title="确定删除该主机?"
            onConfirm={() => handleDelete(r.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button type="link" size="small" danger>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title="主机管理"
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={fetchHosts}>
              刷新
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/dashboard/hosts/new')}>
              添加主机
            </Button>
          </Space>
        }
      >
        <Table
          columns={columns}
          dataSource={hosts}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 20 }}
          locale={{ emptyText: '暂无主机,请添加' }}
        />
      </Card>
    </div>
  )
}

export default HostList
