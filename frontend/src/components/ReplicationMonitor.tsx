import React from 'react'
import { Alert, Card, Col, Descriptions, Row, Statistic, Table, Tag } from 'antd'
import type { ColumnsType } from 'antd/es/table'

const lagColor = (seconds: number) => {
  if (seconds < 0) return '#999'
  if (seconds < 10) return '#52c41a'
  if (seconds < 60) return '#faad14'
  return '#ff4d4f'
}

const ReplTag: React.FC<{ value: boolean | string | undefined; trueLabel?: string; falseLabel?: string }> = ({ value, trueLabel = 'Yes', falseLabel = 'No' }) => {
  const isOk = value === true || value === 'Yes' || value === 'YES' || value === 'ONLINE' || value === 'Primary'
  return <Tag color={isOk ? 'success' : 'error'}>{isOk ? trueLabel : (value === undefined ? '-' : falseLabel)}</Tag>
}

export const ReplicationMonitor: React.FC<{ status: Record<string, any> }> = ({ status }) => {
  const clusterType = (status.cluster_type || 'ha').toLowerCase()
  const queryFailed = status.query_failed === true

  const getField = (snake: string, camel: string) => status[snake] ?? status[camel]

  if (queryFailed) {
    return (
      <div>
        <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
          <Col span={6}>
            <Card size="small">
              <Statistic title="集群架构" value={clusterType.toUpperCase()} valueStyle={{ color: '#1677ff' }} />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic title="连接状态" value="失败" valueStyle={{ color: '#ff4d4f' }} />
            </Card>
          </Col>
        </Row>
        <Alert type="warning" message="无法查询 MySQL 同步状态" description={status.message || '可能原因：实例密码不正确或 MySQL 服务未运行。请在实例管理中编辑实例并更新正确的密码。'} showIcon />
      </div>
    )
  }

  // ---- HA / MHA ----
  if (clusterType === 'ha' || clusterType === 'mha') {
    const ioRunning = getField('slave_io_running', 'Slave_IO_Running')
    const sqlRunning = getField('slave_sql_running', 'Slave_SQL_Running')
    const lagRaw = getField('seconds_behind_master', 'Seconds_Behind_Master')
    const lag = typeof lagRaw === 'number' ? lagRaw : parseInt(lagRaw, 10)
    const isMaster = !ioRunning && !sqlRunning && (isNaN(lag) || lag < 0)

    return (
      <div>
        <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
          <Col span={6}>
            <Card size="small">
              <Statistic title="IO Thread" value={ioRunning ? 'Running' : 'Stopped'} valueStyle={{ color: ioRunning ? '#52c41a' : '#ff4d4f' }} />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic title="SQL Thread" value={sqlRunning ? 'Running' : 'Stopped'} valueStyle={{ color: sqlRunning ? '#52c41a' : '#ff4d4f' }} />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic title="复制延迟" value={lag >= 0 ? `${lag}s` : (isMaster ? 'N/A (主节点)' : '-')} valueStyle={{ color: lagColor(lag) }} />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic title="架构" value={(clusterType || 'HA').toUpperCase()} valueStyle={{ color: '#1677ff' }} />
            </Card>
          </Col>
        </Row>
        {!isMaster && (
          <Descriptions bordered column={2} size="small">
            <Descriptions.Item label="主节点地址">{getField('master_host', 'Master_Host') || '-'}</Descriptions.Item>
            <Descriptions.Item label="主节点端口">{getField('master_port', 'Master_Port') || '-'}</Descriptions.Item>
            <Descriptions.Item label="IO Thread"><ReplTag value={ioRunning} trueLabel="Running" falseLabel="Stopped" /></Descriptions.Item>
            <Descriptions.Item label="SQL Thread"><ReplTag value={sqlRunning} trueLabel="Running" falseLabel="Stopped" /></Descriptions.Item>
            <Descriptions.Item label="复制延迟">{!isNaN(lag) && lag >= 0 ? `${lag} 秒` : '-'}</Descriptions.Item>
            <Descriptions.Item label="Exec Master Log Pos">{getField('exec_master_log_pos', 'Exec_Master_Log_Pos') || '-'}</Descriptions.Item>
            <Descriptions.Item label="Read Master Log Pos">{getField('read_master_log_pos', 'Read_Master_Log_Pos') || '-'}</Descriptions.Item>
            <Descriptions.Item label="Relay Log Space">{getField('relay_log_space', 'Relay_Log_Space') || '-'}</Descriptions.Item>
            <Descriptions.Item label="Retrieved GTID Set" span={2}>
              <span style={{ fontSize: 12, wordBreak: 'break-all' }}>{getField('retrieved_gtid_set', 'Retrieved_Gtid_Set') || '-'}</span>
            </Descriptions.Item>
            <Descriptions.Item label="Executed GTID Set" span={2}>
              <span style={{ fontSize: 12, wordBreak: 'break-all' }}>{getField('executed_gtid_set', 'Executed_Gtid_Set') || '-'}</span>
            </Descriptions.Item>
            {(status.last_error || status.Last_Error) && (
              <Descriptions.Item label="最后错误" span={2}>
                <Alert type="error" message={status.last_error || status.Last_Error} banner />
              </Descriptions.Item>
            )}
          </Descriptions>
        )}
        {isMaster && (
          <Alert type="success" message="当前实例为主节点，无需复制状态监控" showIcon />
        )}
      </div>
    )
  }

  // ---- MGR ----
  if (clusterType === 'mgr') {
    const groupSize = status.group_size || 0
    const onlineMembers = status.online_members || 0
    const isHealthy = onlineMembers === groupSize && groupSize > 0

    const members = Object.entries(status)
      .filter(([k]) => k.startsWith('member_') && k !== 'member_state' && k !== 'member_role')
      .map(([k, v]) => {
        const parts = String(v).split(':')
        return { key: k, host: parts[0] || '-', port: parts[1] || '-', state: parts[2] || '-', role: parts[3] || '-' }
      })

    return (
      <div>
        <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
          <Col span={6}>
            <Card size="small">
              <Statistic title="组状态" value={isHealthy ? 'Healthy' : 'Degraded'} valueStyle={{ color: isHealthy ? '#52c41a' : '#ff4d4f' }} />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic title="在线/总数" value={`${onlineMembers}/${groupSize}`} valueStyle={{ color: isHealthy ? '#52c41a' : '#faad14' }} />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic title="Primary 节点" value={status.primary_member || '-'} valueStyle={{ fontSize: 14 }} />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic title="架构" value="MGR" valueStyle={{ color: '#1677ff' }} />
            </Card>
          </Col>
        </Row>
        <Table
          size="small"
          pagination={false}
          dataSource={members}
          rowKey="key"
          columns={[
            { title: '节点', dataIndex: 'host', key: 'host', render: (v: string, r: any) => `${v}:${r.port}` },
            { title: '状态', dataIndex: 'state', key: 'state', render: (v: string) => <ReplTag value={v} trueLabel={v} falseLabel={v} /> },
            { title: '角色', dataIndex: 'role', key: 'role', render: (v: string) => <Tag color={v === 'PRIMARY' ? 'blue' : 'default'}>{v}</Tag> },
          ]}
        />
        {status.transactions_applied && (
          <div style={{ marginTop: 12, color: '#888', fontSize: 12 }}>已应用事务: {status.transactions_applied}</div>
        )}
      </div>
    )
  }

  // ---- PXC ----
  if (clusterType === 'pxc') {
    const clusterStatus = status.wsrep_cluster_status || '-'
    const localState = status.wsrep_local_state_comment || status.wsrep_local_state || '-'
    const wsrepReady = status.wsrep_ready
    const clusterSize = status.wsrep_cluster_size || 0
    const flowControl = status.wsrep_flow_control_paused

    return (
      <div>
        <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
          <Col span={6}>
            <Card size="small">
              <Statistic title="集群状态" value={clusterStatus} valueStyle={{ color: clusterStatus === 'Primary' ? '#52c41a' : '#ff4d4f' }} />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic title="集群节点数" value={clusterSize} />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic title="wsrep_ready" value={wsrepReady ? 'Ready' : 'Not Ready'} valueStyle={{ color: wsrepReady ? '#52c41a' : '#ff4d4f' }} />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic title="架构" value="PXC" valueStyle={{ color: '#1677ff' }} />
            </Card>
          </Col>
        </Row>
        <Descriptions bordered column={2} size="small">
          <Descriptions.Item label="集群状态"><Tag color={clusterStatus === 'Primary' ? 'success' : 'error'}>{clusterStatus}</Tag></Descriptions.Item>
          <Descriptions.Item label="本地状态">{localState}</Descriptions.Item>
          <Descriptions.Item label="wsrep_ready"><ReplTag value={wsrepReady} trueLabel="Ready" falseLabel="Not Ready" /></Descriptions.Item>
          <Descriptions.Item label="集群大小">{clusterSize}</Descriptions.Item>
          <Descriptions.Item label="流控暂停">{flowControl !== undefined ? `${(Number(flowControl) * 100).toFixed(1)}%` : '-'}</Descriptions.Item>
          <Descriptions.Item label="本地接收队列">{status.wsrep_local_recv_queue ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="集群配置ID">{status.wsrep_cluster_conf_id || '-'}</Descriptions.Item>
        </Descriptions>
      </div>
    )
  }

  return <Alert type="warning" message={`未知架构类型: ${clusterType}`} showIcon />
}

export default ReplicationMonitor
