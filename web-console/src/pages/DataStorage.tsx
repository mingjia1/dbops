import React, { useEffect, useState } from 'react'
import {
  Card, Row, Col, Statistic, Tag, Space, Button, Alert, Spin, Table,
  Modal, Progress, Typography, Tooltip,
} from 'antd'
import {
  DatabaseOutlined, CloudServerOutlined, ImportOutlined,
  SwapOutlined, ReloadOutlined, CheckCircleOutlined, CloseCircleOutlined,
  FileTextOutlined, HddOutlined,
} from '@ant-design/icons'
import { dataMigrationApi, DataMigrationStatus, MigrateResult } from '../services/api'

const { Title, Text } = Typography

const DataStoragePage: React.FC = () => {
  const [status, setStatus] = useState<DataMigrationStatus | null>(null)
  const [loading, setLoading] = useState(false)
  const [migrating, setMigrating] = useState(false)
  const [importing, setImporting] = useState(false)
  const [migrateResult, setMigrateResult] = useState<MigrateResult | null>(null)
  const [migrateModalOpen, setMigrateModalOpen] = useState(false)

  const load = async () => {
    setLoading(true)
    try {
      const r = await dataMigrationApi.getStatus()
      if (r.code === 200) setStatus(r.data)
    } catch (e: any) {
      console.error(e)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const handleImport = async () => {
    Modal.confirm({
      title: '导入旧 JSON 数据',
      content: '会把 data/hosts.json / data/instances.json 导入到 SQLite, 仅在对应表为空时生效. 已存在的 JSON 文件会被重命名为 *.imported-<时间戳>.',
      okText: '开始导入',
      onOk: async () => {
        setImporting(true)
        try {
          const r = await dataMigrationApi.importLegacyJSON()
          if (r.code === 200) {
            Modal.success({
              title: '导入完成',
              content: `共导入 ${r.data.imported} 条记录`,
            })
            load()
          } else {
            Modal.error({ title: '导入失败', content: r.code + '' })
          }
        } finally {
          setImporting(false)
        }
      },
    })
  }

  const handleMigrate = async () => {
    if (!status?.mysql_configured) {
      Modal.warning({
        title: '未配置 MySQL',
        content: '请先在 platform-backend/config/config.yaml 中设置 database_url (MySQL DSN)',
      })
      return
    }
    setMigrating(true)
    setMigrateResult(null)
    setMigrateModalOpen(true)
    try {
      const r = await dataMigrationApi.migrateToMySQL()
      if (r.code === 200 && r.data) {
        setMigrateResult(r.data)
      } else {
        Modal.error({ title: '迁移失败', content: r.message })
        setMigrateModalOpen(false)
      }
    } catch (e: any) {
      Modal.error({ title: '迁移失败', content: e?.message || String(e) })
      setMigrateModalOpen(false)
    } finally {
      setMigrating(false)
    }
  }

  const dialectInfo = (() => {
    if (!status) return null
    if (status.dialect === 'mysql') {
      return { color: 'green', label: 'MySQL', icon: <CloudServerOutlined />, desc: '主库模式, 适合生产环境, 多实例可共享' }
    }
    return { color: 'blue', label: 'SQLite', icon: <HddOutlined />, desc: '本地嵌入式数据库, 零配置启动, 适合开发/单机部署' }
  })()

  const totalRows = status ? Object.values(status.row_counts).reduce((a, b) => a + b, 0) : 0

  return (
    <div style={{ padding: 24 }}>
      <Title level={3}>
        <DatabaseOutlined style={{ marginRight: 8 }} />
        数据存储管理
      </Title>
      <Text type="secondary">
        平台支持 SQLite / MySQL 双驱动. SQLite 零配置即用, MySQL 适合生产多机部署.
      </Text>

      <Spin spinning={loading}>
        {status && dialectInfo && (
          <>
            <Row gutter={16} style={{ marginTop: 24 }}>
              <Col span={6}>
                <Card>
                  <Statistic
                    title="当前存储后端"
                    value={dialectInfo.label}
                    prefix={dialectInfo.icon}
                    valueStyle={{ color: dialectInfo.color === 'green' ? '#52c41a' : '#1890ff' }}
                  />
                  <Tag color={dialectInfo.color} style={{ marginTop: 8 }}>{status.dialect}</Tag>
                </Card>
              </Col>
              <Col span={6}>
                <Card>
                  <Statistic title="总记录数" value={totalRows} prefix={<FileTextOutlined />} />
                  <Text type="secondary" style={{ fontSize: 12 }}>跨 {Object.keys(status.row_counts).length} 张表</Text>
                </Card>
              </Col>
              <Col span={6}>
                <Card>
                  <Statistic
                    title="SQLite 文件"
                    value={status.sqlite_path || '未设置'}
                    valueStyle={{ fontSize: 14 }}
                  />
                  <Text type="secondary" style={{ fontSize: 12 }}>本地持久化路径</Text>
                </Card>
              </Col>
              <Col span={6}>
                <Card>
                  <Statistic
                    title="MySQL DSN"
                    value={status.mysql_configured ? '已配置' : '未配置'}
                    valueStyle={{ color: status.mysql_configured ? '#52c41a' : '#999' }}
                    prefix={status.mysql_configured ? <CheckCircleOutlined /> : <CloseCircleOutlined />}
                  />
                  <Text type="secondary" style={{ fontSize: 12 }}>config.yaml database_url</Text>
                </Card>
              </Col>
            </Row>

            <Alert
              style={{ marginTop: 16 }}
              type="info"
              showIcon
              message={dialectInfo.desc}
            />

            <Card
              title={<><FileTextOutlined /> 各表数据量</>}
              style={{ marginTop: 16 }}
              extra={<Button icon={<ReloadOutlined />} onClick={load}>刷新</Button>}
            >
              <Table
                size="small"
                pagination={false}
                rowKey="table"
                dataSource={Object.entries(status.row_counts).map(([k, v]) => ({
                  table: k,
                  rows: v,
                  hasData: v > 0,
                }))}
                columns={[
                  { title: '表名', dataIndex: 'table', key: 'table' },
                  {
                    title: '记录数', dataIndex: 'rows', key: 'rows', width: 200,
                    render: (v: number) => v === 0
                      ? <Text type="secondary">0</Text>
                      : <Tag color="blue">{v} 条</Tag>,
                  },
                  {
                    title: '状态', dataIndex: 'hasData', key: 'hasData', width: 200,
                    render: (v: boolean) => v
                      ? <Progress percent={100} size="small" showInfo={false} status="active" />
                      : <Progress percent={0} size="small" showInfo={false} />,
                  },
                ]}
              />
            </Card>

            <Card title={<><SwapOutlined /> 数据迁移</>} style={{ marginTop: 16 }}>
              <Row gutter={16}>
                <Col span={12}>
                  <Card type="inner" title="JSON → SQLite">
                    <Space direction="vertical" style={{ width: '100%' }}>
                      <Text>从旧的 data/*.json 文件导入到 SQLite. 仅在对应表为空时执行, 已存在文件自动归档为 *.imported-时间戳.</Text>
                      <Button
                        type="primary"
                        icon={<ImportOutlined />}
                        loading={importing}
                        onClick={handleImport}
                        disabled={status.dialect === 'mysql'}
                      >
                        执行导入
                      </Button>
                    </Space>
                  </Card>
                </Col>
                <Col span={12}>
                  <Card type="inner" title="SQLite → MySQL">
                    <Space direction="vertical" style={{ width: '100%' }}>
                      <Text>把当前 SQLite 数据全量搬到 MySQL. 走 INSERT ... ON DUPLICATE KEY UPDATE, 重复执行安全.</Text>
                      <Tooltip title={!status.mysql_configured ? '请先在 config.yaml 配置 database_url' : ''}>
                        <Button
                          type="primary"
                          danger={status.dialect === 'sqlite'}
                          icon={<SwapOutlined />}
                          loading={migrating}
                          onClick={handleMigrate}
                          disabled={!status.mysql_configured}
                        >
                          执行迁移
                        </Button>
                      </Tooltip>
                    </Space>
                  </Card>
                </Col>
              </Row>
            </Card>
          </>
        )}
      </Spin>

      <Modal
        title="迁移结果"
        open={migrateModalOpen}
        onCancel={() => setMigrateModalOpen(false)}
        footer={[<Button key="close" onClick={() => setMigrateModalOpen(false)}>关闭</Button>]}
        width={760}
      >
        {migrating && <Spin tip="正在迁移..." />}
        {migrateResult && (
          <>
            <Alert
              style={{ marginBottom: 16 }}
              type={migrateResult.total_rows > 0 ? 'success' : 'info'}
              showIcon
              message={`共迁移 ${migrateResult.total_rows} 行, 用时 ${migrateResult.duration_ms}ms`}
            />
            <Table
              size="small"
              rowKey="table"
              pagination={false}
              dataSource={migrateResult.tables}
              columns={[
                { title: '表', dataIndex: 'table', key: 'table' },
                { title: '行数', dataIndex: 'rows', key: 'rows', width: 100,
                  render: (v: number) => v > 0 ? <Tag color="blue">{v}</Tag> : <Text type="secondary">0</Text> },
                { title: '状态', dataIndex: 'status', key: 'status', width: 100,
                  render: (v: string) => {
                    if (v === 'ok') return <Tag color="green"><CheckCircleOutlined /> ok</Tag>
                    if (v === 'skipped') return <Tag color="default">skipped</Tag>
                    return <Tag color="red"><CloseCircleOutlined /> failed</Tag>
                  },
                },
                { title: '备注', dataIndex: 'message', key: 'message' },
              ]}
            />
          </>
        )}
      </Modal>
    </div>
  )
}

export default DataStoragePage
