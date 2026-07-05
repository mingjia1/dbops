import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Space, Tag, message, Descriptions, Popconfirm, Typography } from 'antd'
import { KeyOutlined, ReloadOutlined, HistoryOutlined } from '@ant-design/icons'
import api from '../services/api'

const { Text } = Typography

interface KeyVersion {
  id: string
  version: number
  key_digest: string
  created_at: string
  note?: string
}

export default function KeyRotationPage() {
  const [versions, setVersions] = useState<KeyVersion[]>([])
  const [loading, setLoading] = useState(false)
  const [rotateOpen, setRotateOpen] = useState(false)
  const [rotating, setRotating] = useState(false)

  const fetchVersions = async () => {
    setLoading(true)
    try {
      const res = await api.get('/keys/versions')
      setVersions(res.data || [])
    } catch { message.error('获取密钥版本失败') }
    finally { setLoading(false) }
  }

  useEffect(() => { fetchVersions() }, [])

  const handleRotate = async (note?: string) => {
    setRotating(true)
    try {
      await api.post('/keys/rotate', { note })
      message.success('密钥轮转成功')
      setRotateOpen(false)
      fetchVersions()
    } catch (err: any) { message.error(`轮转失败: ${err.message}`) }
    finally { setRotating(false) }
  }

  const columns = [
    { title: '版本', dataIndex: 'version', key: 'version', width: 80, render: (v: number) => <Tag color="blue">v{v}</Tag> },
    { title: '密钥摘要', dataIndex: 'key_digest', key: 'key_digest', ellipsis: true },
    { title: '备注', dataIndex: 'note', key: 'note', ellipsis: true },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180, render: (v: string) => new Date(v).toLocaleString() },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title={<><KeyOutlined /> 密钥管理</>} extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchVersions}>刷新</Button>
          <Popconfirm title="确认轮转密钥？此操作将生成新的加密密钥。" onConfirm={() => handleRotate()}>
            <Button type="primary" icon={<KeyOutlined />}>轮转密钥</Button>
          </Popconfirm>
        </Space>
      }>
        <Descriptions bordered size="small" style={{ marginBottom: 16 }}>
          <Descriptions.Item label="当前版本">v{versions.length || 0}</Descriptions.Item>
          <Descriptions.Item label="总版本数">{versions.length}</Descriptions.Item>
        </Descriptions>
        <Table columns={columns} dataSource={versions} rowKey="id" loading={loading} size="small" pagination={false} />
      </Card>
    </div>
  )
}
