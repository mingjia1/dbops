import React, { useEffect, useState, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, Table, Button, Space, Tag, Popconfirm, message, Tooltip, Empty } from 'antd'
import { PlusOutlined, ReloadOutlined, ScanOutlined, DesktopOutlined, DatabaseOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { hostApi, instanceApi, type Host, type HostScanResult } from '../services/api'

const HostList: React.FC = () => {
  const navigate = useNavigate()
  const [hosts, setHosts] = useState<Host[]>([])
  const [loading, setLoading] = useState(false)
  const [instanceCount, setInstanceCount] = useState<Record<string, number>>({})
  const [scanningHosts, setScanningHosts] = useState<Record<string, boolean>>({})
  const pollRef = useRef<Record<string, number>>({})

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

  const fetchInstanceCount = async (hostId: string) => {
    try {
      const r: any = await instanceApi.listByHost(hostId, 1, 0)
      const total = r?.total ?? r?.data?.length ?? (Array.isArray(r?.data) ? r.data.length : 0)
      setInstanceCount((p) => ({ ...p, [hostId]: total }))
    } catch {
      setInstanceCount((p) => ({ ...p, [hostId]: 0 }))
    }
  }

  useEffect(() => {
    fetchHosts()
  }, [])

  useEffect(() => {
    hosts.forEach((h) => fetchInstanceCount(h.id))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hosts.length])

  useEffect(() => () => {
    Object.values(pollRef.current).forEach((t) => window.clearInterval(t))
  }, [])

  const stopScanPolling = (hostId: string) => {
    const t = pollRef.current[hostId]
    if (t) {
      window.clearInterval(t)
      delete pollRef.current[hostId]
    }
    setScanningHosts((p) => ({ ...p, [hostId]: false }))
  }

  const startScanPolling = (hostId: string, taskId: string) => {
    const interval = window.setInterval(async () => {
      try {
        const r: any = await hostApi.getScanResult(hostId, taskId)
        const data: HostScanResult = r?.data
        if (!data) return
        if (data.status === 'success') {
          stopScanPolling(hostId)
          const unManaged = (data.instances || []).filter((i) => !i.already_managed)
          if (unManaged.length === 0) {
            message.success(`主机扫描完成, 全部 ${data.instances.length} 个实例已纳管`)
            navigate(`/dashboard/hosts/${hostId}?tab=instances`)
          } else {
            message.success(`发现 ${unManaged.length} 个新实例, 请在主机详情查看`)
            navigate(`/dashboard/hosts/${hostId}?tab=instances&scan_task=${taskId}`)
          }
        } else if (data.status === 'failed') {
          stopScanPolling(hostId)
          message.error(`扫描失败: ${data.error || data.message || '未知错误'}`)
        }
      } catch {
        // ignore poll errors
      }
    }, 2000)
    pollRef.current[hostId] = interval
  }

  const handleScan = async (host: Host) => {
    try {
      setScanningHosts((p) => ({ ...p, [host.id]: true }))
      const r: any = await hostApi.scanInstances(host.id)
      const taskId = r?.data?.task_id
      if (!taskId) {
        message.warning('后端未实现 scan-instances 接口, 请手动添加实例')
        navigate(`/dashboard/instances?preset_host=${host.id}`)
        setScanningHosts((p) => ({ ...p, [host.id]: false }))
        return
      }
      message.info(`已发起扫描: ${host.name}, 正在 SSH 探测中...`)
      startScanPolling(host.id, taskId)
    } catch (err: any) {
      setScanningHosts((p) => ({ ...p, [host.id]: false }))
      if (err?.response?.status === 404) {
        message.warning('后端未实现 scan-instances 接口, 请手动添加实例')
        navigate(`/dashboard/instances?preset_host=${host.id}`)
      } else {
        message.error('扫描发起失败')
      }
    }
  }

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
      title: '操作系统',
      dataIndex: 'os_type',
      key: 'os_type',
      render: (os) => os?.toUpperCase() || '-',
    },
    {
      title: '实例数',
      key: 'instances',
      render: (_, r) => {
        const n = instanceCount[r.id]
        if (n === undefined) return <Tag>未加载</Tag>
        if (n === 0) {
          return (
            <Tooltip title="该主机暂无已纳管实例, 建议自动扫描或手动添加">
              <Tag color="warning" icon={<DatabaseOutlined />}>0</Tag>
            </Tooltip>
          )
        }
        return <Tag color="processing" icon={<DatabaseOutlined />}>{n}</Tag>
      },
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
      width: 280,
      render: (_, r) => (
        <Space>
          <Button type="link" size="small" onClick={() => navigate(`/dashboard/hosts/${r.id}`)}>
            详情
          </Button>
          <Button
            type="link"
            size="small"
            icon={<ScanOutlined />}
            loading={!!scanningHosts[r.id]}
            onClick={() => handleScan(r)}
          >
            扫描实例
          </Button>
          <Button type="link" size="small" onClick={() => navigate(`/dashboard/instances?host_id=${r.id}`)}>
            管理实例
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
        title={
          <Space>
            <DesktopOutlined />
            <span>主机 & 实例管理</span>
          </Space>
        }
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
          locale={{
            emptyText: (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={
                  <div>
                    <div style={{ marginBottom: 8 }}>暂无主机</div>
                    <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/dashboard/hosts/new')}>
                      添加第一台主机
                    </Button>
                  </div>
                }
              />
            ),
          }}
        />
      </Card>
    </div>
  )
}

export default HostList
