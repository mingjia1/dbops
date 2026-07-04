import React from 'react'
import { Alert, Form, FormInstance, Input, Modal, Switch } from 'antd'

interface AutoFailoverModalProps {
  open: boolean
  confirming: boolean
  form: FormInstance
  forceMode: boolean | undefined
  onOk: () => void
  onCancel: () => void
}

export const AutoFailoverModal: React.FC<AutoFailoverModalProps> = ({
  open, confirming, form, forceMode, onOk, onCancel,
}) => (
  <Modal
    title="自动故障转移"
    open={open}
    onCancel={onCancel}
    onOk={onOk}
    confirmLoading={confirming}
    okText="确认执行"
    cancelText="取消"
    okButtonProps={{ danger: true }}
  >
    <Form form={form} layout="vertical">
      <Alert
        type="error"
        showIcon
        message="自动故障转移只在系统确认主节点故障后执行"
        description="自动故障转移不需要输入触发实例 ID，后端会根据当前集群主节点和故障确认状态决定是否切换。"
        style={{ marginBottom: 12 }}
      />
      <Form.Item name="reason" label="故障原因" rules={[{ required: true, message: '请输入故障原因' }]}>
        <Input placeholder="例如：主库宕机" />
      </Form.Item>
      <Form.Item name="force" label="强制执行" valuePropName="checked">
        <Switch checkedChildren="强制" unCheckedChildren="安全模式" />
      </Form.Item>
      {forceMode && (
        <Alert type="warning" showIcon message="强制模式可能绕过保护检查，请确认业务和数据风险。" style={{ marginBottom: 12 }} />
      )}
      <Form.Item name="confirm_impact" valuePropName="checked" rules={[{ required: true, message: '请确认操作影响' }]}>
        <Switch checkedChildren="已确认影响" unCheckedChildren="未确认" />
      </Form.Item>
    </Form>
  </Modal>
)
