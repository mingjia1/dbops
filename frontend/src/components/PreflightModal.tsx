import React from 'react'
import { Alert, Button, Col, Descriptions, Modal, Row, Spin, Statistic } from 'antd'
import type { PreflightResult } from '../services/haHelpers'

interface PreflightModalProps {
  open: boolean
  loading: boolean
  result: PreflightResult | null
  onClose: () => void
}

export const PreflightModal: React.FC<PreflightModalProps> = ({ open, loading, result, onClose }) => {
  const pass = !!result?.pass

  return (
    <Modal
      title="Pre-flight 检查"
      open={open}
      onCancel={onClose}
      footer={<Button onClick={onClose}>关闭</Button>}
      width={760}
    >
      {loading ? (
        <div style={{ textAlign: 'center', padding: 30 }}><Spin /></div>
      ) : result ? (
        <>
          <Row gutter={16} style={{ marginBottom: 16 }}>
            <Col span={6}>
              <Statistic title="主节点健康" value={result.current_master_healthy ? '是' : '否'}
                valueStyle={{ color: result.current_master_healthy ? '#3f8600' : '#cf1322' }} />
            </Col>
            <Col span={6}>
              <Statistic title="健康从节点" value={`${result.healthy_slave_count} / ${result.slave_count}`} />
            </Col>
            <Col span={6}>
              <Statistic title="最大复制延迟" value={result.max_replication_lag} suffix="s"
                valueStyle={{ color: result.max_replication_lag > 30 ? '#cf1322' : '#3f8600' }} />
            </Col>
            <Col span={6}>
              <Statistic title="GTID 一致" value={result.gtid_consistent ? '是' : '否'}
                valueStyle={{ color: result.gtid_consistent ? '#3f8600' : '#cf1322' }} />
            </Col>
          </Row>
          <Descriptions bordered size="small" column={2} style={{ marginBottom: 16 }}>
            <Descriptions.Item label="平台主节点">{result.platform_primary_id || result.current_master_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="真实主节点">{result.real_primary_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="目标新主">{result.target_master_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="拓扑一致">{result.topology_consistent ? '是' : '否'}</Descriptions.Item>
          </Descriptions>
          <Alert
            type={pass ? 'success' : 'error'}
            showIcon
            message={pass ? '检查通过，可以在非强制模式下切换' : '检查未通过，非强制模式不允许切换'}
            description={
              <div>
                {(result.blocking_reasons?.length || 0) > 0 && (
                  <ul style={{ marginBottom: 8, paddingLeft: 18 }}>
                    {result.blocking_reasons?.map((item, index) => <li key={`block-${index}`}>{item}</li>)}
                  </ul>
                )}
                {(result.warnings?.length || 0) > 0 && (
                  <ul style={{ marginBottom: 0, paddingLeft: 18 }}>
                    {result.warnings?.map((item, index) => <li key={`warn-${index}`}>{item}</li>)}
                  </ul>
                )}
              </div>
            }
          />
        </>
      ) : null}
    </Modal>
  )
}
