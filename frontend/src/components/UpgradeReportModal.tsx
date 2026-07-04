import React from 'react'
import { Modal, Spin, Button, Space, Descriptions, Alert, Card, Typography, Empty } from 'antd'

const { Paragraph } = Typography

interface UpgradeReportModalProps {
  open: boolean
  loading: boolean
  result: any | null
  onClose: () => void
}

const UpgradeReportModal: React.FC<UpgradeReportModalProps> = ({ open, loading, result, onClose }) => (
  <Modal
    title="升级报告"
    open={open}
    onCancel={onClose}
    footer={<Button onClick={onClose}>关闭</Button>}
    width={760}
  >
    <Spin spinning={loading}>
      {result ? (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Descriptions size="small" bordered column={1}>
            <Descriptions.Item label="报告 ID">{result.report_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="任务 / 计划 ID">{result.plan_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="生成时间">{result.generated_at || '-'}</Descriptions.Item>
            <Descriptions.Item label="摘要">{result.summary || '-'}</Descriptions.Item>
            <Descriptions.Item label="详情">{result.details || '-'}</Descriptions.Item>
          </Descriptions>
          <Descriptions size="small" bordered column={2} title="指标">
            <Descriptions.Item label="耗时(秒)">{result.metrics?.duration_seconds ?? 0}</Descriptions.Item>
            <Descriptions.Item label="错误">{result.metrics?.errors_encountered ?? 0}</Descriptions.Item>
            <Descriptions.Item label="警告">{result.metrics?.warnings_generated ?? 0}</Descriptions.Item>
            <Descriptions.Item label="表数量">{result.metrics?.tables_processed ?? 0}</Descriptions.Item>
          </Descriptions>
          {(result.issues || []).map((item: any, index: number) => (
            <Alert
              key={index}
              type={item.severity === 'error' ? 'error' : 'warning'}
              message={`${item.type || '问题'}: ${item.description || '-'}`}
              description={`是否解决=${item.resolved ? '是' : '否'} ${item.timestamp || ''}`}
            />
          ))}
          {(result.recommendations || []).length > 0 && (
            <Card size="small" title="建议">
              {(result.recommendations || []).map((item: string, index: number) => (
                <Paragraph key={index}>{item}</Paragraph>
              ))}
            </Card>
          )}
        </Space>
      ) : (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} />
      )}
    </Spin>
  </Modal>
)

export default UpgradeReportModal
