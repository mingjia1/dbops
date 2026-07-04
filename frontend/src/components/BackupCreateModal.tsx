import React from 'react'
import { Form, Modal, Radio } from 'antd'

interface BackupCreateModalProps {
  open: boolean
  submitting: boolean
  form: any
  onOk: () => void
  onCancel: () => void
}

const BackupCreateModal: React.FC<BackupCreateModalProps> = ({
  open, submitting, form, onOk, onCancel,
}) => (
  <Modal
    title="创建备份任务"
    open={open}
    onCancel={onCancel}
    onOk={onOk}
    confirmLoading={submitting}
    okText="创建任务"
    cancelText="取消"
  >
    <Form form={form} layout="vertical">
      <Form.Item label="备份类型" name="backup_type" rules={[{ required: true }]}>
        <Radio.Group>
          <Radio.Button value="full">全量</Radio.Button>
          <Radio.Button value="incremental">增量</Radio.Button>
          <Radio.Button value="logical">逻辑</Radio.Button>
        </Radio.Group>
      </Form.Item>
    </Form>
  </Modal>
)

export default BackupCreateModal
