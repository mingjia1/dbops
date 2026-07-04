import React from 'react'
import { Alert, Form, Input, Modal, Select } from 'antd'

interface RegisterInstanceModalProps {
  open: boolean
  hostName: string | undefined
  targetPort: number | null
  clusters: any[]
  registering: boolean
  onCancel: () => void
  onSubmit: (values: any) => Promise<void>
}

const RegisterInstanceModal: React.FC<RegisterInstanceModalProps> = ({
  open, hostName, targetPort, clusters, registering, onCancel, onSubmit,
}) => {
  const [form] = Form.useForm()

  React.useEffect(() => {
    if (open) {
      form.resetFields()
      form.setFieldsValue({
        name: `${hostName || 'host'}-${targetPort}`,
        username: 'root',
        port: targetPort,
      })
    }
  }, [open, hostName, targetPort, form])

  const handleOk = async () => {
    const values = await form.validateFields()
    await onSubmit(values)
  }

  return (
    <Modal
      title={`纳管扫描到的实例: ${targetPort || ''}`}
      open={open}
      onCancel={onCancel}
      onOk={handleOk}
      confirmLoading={registering}
      okText="纳管"
      cancelText="取消"
      width={560}
    >
      <Form form={form} layout="vertical">
        <Form.Item name="name" label="实例名称" rules={[{ required: true, message: '请输入实例名称' }]}>
          <Input />
        </Form.Item>
        <Form.Item
          name="username"
          label="连接用户名"
          rules={[{ required: true, message: '请输入连接用户名' }]}
          extra="默认 root, 也可使用具有 SUPER/REPLICATION 权限的运维账号"
        >
          <Input placeholder="例如: root" />
        </Form.Item>
        <Form.Item
          name="password"
          label="连接密码"
          rules={[{ required: true, message: '请输入密码' }]}
        >
          <Input.Password placeholder="MySQL 密码" autoComplete="new-password" />
        </Form.Item>
        <Form.Item name="cluster_id" label="所属集群">
          <Select
            allowClear
            placeholder="选择集群（可选）"
            options={clusters.map((c: any) => ({
              value: c.cluster_id,
              label: `${c.cluster_id} (${c.arch || '未知架构'}) - ${c.node_count || 0}节点`,
            }))}
          />
        </Form.Item>
      </Form>
    </Modal>
  )
}

export default RegisterInstanceModal
