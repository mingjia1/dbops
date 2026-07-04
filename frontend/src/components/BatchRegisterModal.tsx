import React from 'react'
import { Alert, Form, Input, Modal, Select } from 'antd'

interface BatchRegisterModalProps {
  open: boolean
  newInstanceCount: number
  clusters: any[]
  registering: boolean
  onCancel: () => void
  onSubmit: (values: any) => Promise<void>
}

const BatchRegisterModal: React.FC<BatchRegisterModalProps> = ({
  open, newInstanceCount, clusters, registering, onCancel, onSubmit,
}) => {
  const [form] = Form.useForm()

  React.useEffect(() => {
    if (open) {
      form.resetFields()
      form.setFieldsValue({ username: 'root' })
    }
  }, [open, form])

  const handleOk = async () => {
    const values = await form.validateFields()
    await onSubmit(values)
  }

  return (
    <Modal
      title="一键纳管全部待纳管实例"
      open={open}
      onCancel={onCancel}
      onOk={handleOk}
      confirmLoading={registering}
      okText="全部纳管"
      cancelText="取消"
      width={560}
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message={`将纳管 ${newInstanceCount} 个扫描发现的实例`}
        description="这些实例会使用同一组 MySQL 连接账号和密码登记到平台。已纳管端口会自动跳过。"
      />
      <Form form={form} layout="vertical">
        <Form.Item name="username" label="连接用户名" rules={[{ required: true, message: '请输入连接用户名' }]}>
          <Input placeholder="例如: root" />
        </Form.Item>
        <Form.Item name="password" label="连接密码" rules={[{ required: true, message: '请输入密码' }]}>
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

export default BatchRegisterModal
