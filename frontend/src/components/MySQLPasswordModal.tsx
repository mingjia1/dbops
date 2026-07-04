import React from 'react'
import { Alert, Form, Input, Modal, Space } from 'antd'
import { KeyOutlined } from '@ant-design/icons'

interface MySQLPasswordModalProps {
  open: boolean
  onClose: () => void
  onSave: () => void
  form: any
  initialValues?: { username: string; password: string }
}

const MySQLPasswordModal: React.FC<MySQLPasswordModalProps> = ({
  open,
  onClose,
  onSave,
  form,
  initialValues = { username: 'root', password: 'Root#1234' },
}) => {
  return (
    <Modal
      title={
        <Space>
          <KeyOutlined />
          <span>设置 MySQL Root 密码</span>
        </Space>
      }
      open={open}
      onCancel={onClose}
      onOk={onSave}
      okText="保存"
      cancelText="取消"
      destroyOnClose
    >
      <Alert
        type="info"
        showIcon
        message="此密码将用于集群部署时设置 MySQL root 用户密码"
        description="部署过程中会使用此密码初始化 MySQL root 账户。请牢记此密码，部署完成后可使用此密码连接 MySQL。"
        style={{ marginBottom: 16 }}
      />
      <Form form={form} layout="vertical">
        <Form.Item name="username" label="用户名" initialValue={initialValues.username}>
          <Input placeholder="root" disabled />
        </Form.Item>
        <Form.Item
          name="password"
          label="密码"
          rules={[{ required: true, message: '请输入 MySQL root 密码' }]}
          initialValue={initialValues.password}
        >
          <Input.Password placeholder="请输入 MySQL root 密码" />
        </Form.Item>
      </Form>
    </Modal>
  )
}

export default MySQLPasswordModal
