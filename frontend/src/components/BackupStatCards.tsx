import React from 'react'
import { Card, Col, Row, Statistic } from 'antd'
import type { BackupRecord } from '../services/backupHelpers'
import { isCompletedBackupStatus, isActiveBackupStatus, isFailedBackupStatus } from '../services/backupHelpers'

interface BackupStatCardsProps {
  records: BackupRecord[]
}

const BackupStatCards: React.FC<BackupStatCardsProps> = ({ records }) => (
  <Row gutter={16} style={{ marginBottom: 16 }}>
    <Col span={6}>
      <Card size="small">
        <Statistic title="记录数" value={records.length} />
      </Card>
    </Col>
    <Col span={6}>
      <Card size="small">
        <Statistic
          title="已完成"
          value={records.filter((r) => isCompletedBackupStatus(r.status)).length}
          valueStyle={{ color: '#3f8600' }}
        />
      </Card>
    </Col>
    <Col span={6}>
      <Card size="small">
        <Statistic
          title="运行中"
          value={records.filter((r) => isActiveBackupStatus(r.status)).length}
          valueStyle={{ color: '#1677ff' }}
        />
      </Card>
    </Col>
    <Col span={6}>
      <Card size="small">
        <Statistic
          title="失败"
          value={records.filter((r) => isFailedBackupStatus(r.status)).length}
        />
      </Card>
    </Col>
  </Row>
)

export default BackupStatCards
