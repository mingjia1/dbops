import React, { useState, useEffect } from 'react'
import { Alert, Card, Table, Button, Modal, Form, Input, Space, Tag, Descriptions, Popconfirm, Typography } from 'antd'
import { message } from '../components/antdMessage'
import { KeyOutlined, ReloadOutlined } from '@ant-design/icons'
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
  const [lastResult, setLastResult] = useState<string | null>(null)
  const [form] = Form.useForm()

  const fetchVersions = async () => {
    setLoading(true)
    try {
      const res = await api.get('/keys/versions')
      setVersions(res.data || [])
    } catch { message.error('获取密钥版本失败') }
    finally { setLoading(false) }
  }

  useEffect(() => { fetchVersions() }, [])

  const handleRotate = async () => {
    const values = await form.validateFields()
    setRotating(true)
    try {
      const res = await api.post('/keys/rotate', { new_key: values.new_key, note: values.note })
      const count = res.data?.records_re_encrypted ?? '-'
      message.success(`密钥轮转成功, 已重加密 ${count} 条记录`)
      setLastResult(
        `轮转完成: 已重加密 ${count} 条记录。请立即将新密钥写入环境变量 DBOPS_ENCRYPTION_KEY 并重启后端, ` +
        `否则重启后平台将无法解密既有数据。`,
      )
      setRotateOpen(false)
      form.resetFields()
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
          <Button type="primary" icon={<KeyOutlined />} onClick={() => setRotateOpen(true)}>轮转密钥</Button>
        </Space>
      }>
        {lastResult && (
          <Alert type="warning" showIcon message="轮转结果" description={lastResult} style={{ marginBottom: 16 }} closable />
        )}
        <Descriptions bordered size="small" style={{ marginBottom: 16 }}>
          <Descriptions.Item label="已归档版本数">{versions.length}</Descriptions.Item>
        </Descriptions>
        <Table columns={columns} dataSource={versions} rowKey="id" loading={loading} size="small" pagination={false} />
      </Card>
      <Modal
        title="轮转加密密钥"
        open={rotateOpen}
        onCancel={() => setRotateOpen(false)}
        onOk={handleRotate}
        confirmLoading={rotating}
        okText="确认轮转"
        okButtonProps={{ danger: true }}
      >
        <Alert
          type="warning"
          showIcon
          message="高危操作"
          description="轮转会用新密钥重加密平台内全部加密数据(实例密码等)。轮转完成后必须更新 DBOPS_ENCRYPTION_KEY 环境变量并重启后端。"
          style={{ marginBottom: 16 }}
        />
        <Form form={form} layout="vertical">
          <Form.Item
            name="new_key"
            label="新密钥"
            rules={[{ required: true, min: 32, message: '新密钥至少 32 个字符' }]}
          >
            <Input.Password placeholder="至少 32 个字符的强随机密钥" autoComplete="new-password" />
          </Form.Item>
          <Form.Item name="note" label="备注">
            <Input placeholder="如: 2026 季度例行轮转" />
          </Form.Item>
          <Text type="secondary" style={{ fontSize: 12 }}>
            当前运行密钥将被归档为版本记录, 建议轮转前确认已有最新元数据备份。
          </Text>
        </Form>
      </Modal>
    </div>
  )
}
