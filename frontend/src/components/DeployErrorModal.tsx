import React from 'react'
import { Alert, Button, Modal, Space, Table, Tag } from 'antd'
import type { DeployResult } from '../services/deployHelpers'
import { isFailedDeployStatus, stepStatusToAntd } from '../services/deployHelpers'

interface DeployErrorModalProps {
  open: boolean
  detail: DeployResult | null
  onClose: () => void
}

const DeployErrorModal: React.FC<DeployErrorModalProps> = ({ open, detail, onClose }) => {
  return (
    <Modal
      title="部署失败详情"
      open={open}
      onCancel={onClose}
      footer={<Button type="primary" onClick={onClose}>关闭</Button>}
      width={820}
    >
      {detail && (
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Alert
            type="error"
            showIcon
            message={`${detail.cluster_type?.toUpperCase?.() || '集群'} 部署失败`}
            description={<pre style={{ whiteSpace: 'pre-wrap', margin: 0 }}>{detail.message || detail.status}</pre>}
          />
          {detail.steps && detail.steps.length > 0 && (
            <Table
              size="small"
              pagination={false}
              rowKey={(row, index) => `${row.name}-${index}`}
              dataSource={detail.steps}
              columns={[
                { title: '步骤', dataIndex: 'name', key: 'name', width: 220 },
                {
                  title: '状态',
                  dataIndex: 'status',
                  key: 'status',
                  width: 90,
                  render: (status: string) => (
                    <Tag color={isFailedDeployStatus(status) ? 'error' : stepStatusToAntd(status) === 'finish' ? 'success' : 'default'}>{status}</Tag>
                  ),
                },
                { title: '信息', dataIndex: 'message', key: 'message', render: (text: string) => text ? <pre style={{ whiteSpace: 'pre-wrap', margin: 0 }}>{text}</pre> : '-' },
              ]}
            />
          )}
        </Space>
      )}
    </Modal>
  )
}

export default DeployErrorModal
