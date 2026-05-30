import React, { useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, InputNumber, message, Tag, Steps, Card, Progress, Upload, DatePicker, Divider, Typography, Tabs } from 'antd'
import { PlayCircleOutlined, CheckCircleOutlined, SwapOutlined, SyncOutlined, HistoryOutlined, DownloadOutlined, FileTextOutlined, ExclamationCircleOutlined } from '@ant-design/icons'
import type { UploadProps } from 'antd'

const { Title } = Typography
const { TabPane } = Tabs
const { RangePicker } = DatePicker

interface UpgradeHistory {
  id: string
  instance_id: string
  instance_name: string
  upgrade_type: 'in_place' | 'logical' | 'rolling'
  source_version: string
  target_version: string
  status: 'pending' | 'running' | 'success' | 'failed'
  start_time: string
  end_time?: string
  duration?: number
}

const UpgradeManage: React.FC = () => {
  const [planModalVisible, setPlanModalVisible] = useState(false)
  const [compatModalVisible, setCompatModalVisible] = useState(false)
  const [inPlaceModalVisible, setInPlaceModalVisible] = useState(false)
  const [logicalModalVisible, setLogicalModalVisible] = useState(false)
  const [rollingModalVisible, setRollingModalVisible] = useState(false)
  const [reportModalVisible, setReportModalVisible] = useState(false)
  const [compatResult, setCompatResult] = useState<any>(null)
  const [upgradeProgress, setUpgradeProgress] = useState(0)
  const [currentStep, setCurrentStep] = useState(0)

  const [planForm] = Form.useForm()
  const [compatForm] = Form.useForm()
  const [inPlaceForm] = Form.useForm()
  const [logicalForm] = Form.useForm()
  const [rollingForm] = Form.useForm()

  const mockHistory: UpgradeHistory[] = [
    {
      id: '1',
      instance_id: 'inst-001',
      instance_name: 'MySQL-生产-01',
      upgrade_type: 'in_place',
      source_version: '5.7.38',
      target_version: '8.0.32',
      status: 'success',
      start_time: '2024-01-15 10:00:00',
      end_time: '2024-01-15 10:30:00',
      duration: 1800,
    },
    {
      id: '2',
      instance_id: 'inst-002',
      instance_name: 'MySQL-生产-02',
      upgrade_type: 'logical',
      source_version: '5.6.51',
      target_version: '8.0.32',
      status: 'running',
      start_time: '2024-01-16 14:00:00',
    },
    {
      id: '3',
      instance_id: 'inst-003',
      instance_name: 'MySQL-测试-01',
      upgrade_type: 'rolling',
      source_version: '5.7.42',
      target_version: '8.0.33',
      status: 'failed',
      start_time: '2024-01-17 09:00:00',
      end_time: '2024-01-17 09:15:00',
      duration: 900,
    },
  ]

  const handlePlanUpgradePath = async (values: any) => {
    message.success('升级路径规划已生成')
    setPlanModalVisible(false)
    planForm.resetFields()
  }

  const handleCheckCompatibility = async (values: any) => {
    message.loading('正在检查兼容性...', 0)
    setTimeout(() => {
      message.destroy()
      setCompatResult({
        compatible: true,
        warnings: [
          '检测到使用了已废弃的 SQL_MODE: NO_AUTO_CREATE_USER',
          '存在 MySQL 5.6 不支持的保留字作为表名',
          '部分存储过程使用了新版本不支持的语法',
        ],
        recommendations: [
          '建议升级前修改 SQL_MODE',
          '建议重命名使用保留字的表',
          '建议先在测试环境验证存储过程',
        ],
        sql_mode_changes: [
          { old: 'NO_AUTO_CREATE_USER', new: '已移除', impact: '需要手动迁移用户权限' },
        ],
        deprecated_features: [
          { feature: 'QUERY_CACHE', action: '需要禁用或移除相关配置' },
        ],
      })
    }, 1500)
  }

  const handleInPlaceUpgrade = async (values: any) => {
    message.loading('正在启动原地升级...', 0)
    setCurrentStep(0)
    const steps = [
      '停止 MySQL 服务',
      '备份数据目录',
      '替换二进制文件',
      '启动 MySQL 服务',
      '执行 mysql_upgrade',
      '验证升级结果',
    ]
    for (let i = 0; i < steps.length; i++) {
      await new Promise(resolve => setTimeout(resolve, 1000))
      setCurrentStep(i + 1)
      setUpgradeProgress(Math.round(((i + 1) / steps.length) * 100))
    }
    message.destroy()
    message.success('原地升级完成')
    setInPlaceModalVisible(false)
    inPlaceForm.resetFields()
    setUpgradeProgress(0)
    setCurrentStep(0)
  }

  const handleLogicalMigration = async (values: any) => {
    message.loading('正在启动逻辑迁移...', 0)
    setCurrentStep(0)
    const steps = [
      '创建目标实例',
      '导出源库数据',
      '传输数据文件',
      '导入目标库',
      '校验数据一致性',
      '切换应用连接',
    ]
    for (let i = 0; i < steps.length; i++) {
      await new Promise(resolve => setTimeout(resolve, 1000))
      setCurrentStep(i + 1)
      setUpgradeProgress(Math.round(((i + 1) / steps.length) * 100))
    }
    message.destroy()
    message.success('逻辑迁移完成')
    setLogicalModalVisible(false)
    logicalForm.resetFields()
    setUpgradeProgress(0)
    setCurrentStep(0)
  }

  const handleRollingUpgrade = async (values: any) => {
    message.loading('正在启动滚动升级...', 0)
    setCurrentStep(0)
    const steps = [
      '选择从节点升级',
      '升级从节点 1',
      '验证从节点 1',
      '主从切换',
      '升级原主节点',
      '验证集群状态',
    ]
    for (let i = 0; i < steps.length; i++) {
      await new Promise(resolve => setTimeout(resolve, 1000))
      setCurrentStep(i + 1)
      setUpgradeProgress(Math.round(((i + 1) / steps.length) * 100))
    }
    message.destroy()
    message.success('滚动升级完成')
    setRollingModalVisible(false)
    rollingForm.resetFields()
    setUpgradeProgress(0)
    setCurrentStep(0)
  }

  const handleDownloadReport = () => {
    const reportContent = `
MySQL 升级报告
生成时间: ${new Date().toLocaleString()}
=====================================

1. 升级路径规划
   - 源版本: 5.7.38
   - 目标版本: 8.0.32
   - 升级类型: 原地升级

2. 兼容性检查结果
   - 状态: 通过 (有警告)
   - 警告项: 3
   - 建议: SQL_MODE 需要调整

3. 执行步骤
   - 停止服务: 成功
   - 备份数据: 成功
   - 升级二进制: 成功
   - 启动服务: 成功
   - 验证结果: 成功

4. 性能对比
   - 升级前 TPS: 12,000
   - 升级后 TPS: 15,000
   - 提升: 25%
`
    const blob = new Blob([reportContent], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `upgrade-report-${Date.now()}.txt`
    a.click()
    URL.revokeObjectURL(url)
    message.success('报告下载成功')
  }

  const renderUpgradeSteps = () => {
    const steps = [
      '准备阶段',
      '备份阶段',
      '升级阶段',
      '验证阶段',
      '完成',
    ]
    return (
      <Steps current={currentStep} size="small">
        {steps.map((title, index) => (
          <Steps.Step key={index} title={title} />
        ))}
      </Steps>
    )
  }

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
    },
    {
      title: '实例名称',
      dataIndex: 'instance_name',
      key: 'instance_name',
    },
    {
      title: '升级类型',
      dataIndex: 'upgrade_type',
      key: 'upgrade_type',
      render: (type: string) => {
        const typeMap: Record<string, { color: string; text: string }> = {
          in_place: { color: 'blue', text: '原地升级' },
          logical: { color: 'green', text: '逻辑迁移' },
          rolling: { color: 'orange', text: '滚动升级' },
        }
        return <Tag color={typeMap[type]?.color}>{typeMap[type]?.text}</Tag>
      },
    },
    {
      title: '版本变化',
      key: 'version',
      render: (_: any, record: UpgradeHistory) => (
        <span>
          {record.source_version} <SwapOutlined /> {record.target_version}
        </span>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        const statusMap: Record<string, { color: string; text: string }> = {
          pending: { color: 'default', text: '待执行' },
          running: { color: 'processing', text: '执行中' },
          success: { color: 'success', text: '成功' },
          failed: { color: 'error', text: '失败' },
        }
        return <Tag color={statusMap[status]?.color}>{statusMap[status]?.text}</Tag>
      },
    },
    {
      title: '开始时间',
      dataIndex: 'start_time',
      key: 'start_time',
      width: 180,
    },
    {
      title: '持续时间',
      dataIndex: 'duration',
      key: 'duration',
      render: (duration: number) => (duration ? `${Math.floor(duration / 60)}分${duration % 60}秒` : '-'),
    },
    {
      title: '操作',
      key: 'action',
      width: 150,
      render: (_: any, record: UpgradeHistory) => (
        <Space>
          <Button size="small" icon={<FileTextOutlined />} onClick={() => setReportModalVisible(true)}>
            报告
          </Button>
          {record.status === 'failed' && (
            <Button size="small" type="primary" ghost icon={<SyncOutlined />}>
              重试
            </Button>
          )}
        </Space>
      ),
    },
  ]

  const uploadProps: UploadProps = {
    name: 'file',
    action: '/api/upload',
    accept: '.sql,.sql.gz',
    onChange(info) {
      if (info.file.status === 'done') {
        message.success(`${info.file.name} 上传成功`)
      } else if (info.file.status === 'error') {
        message.error(`${info.file.name} 上传失败`)
      }
    },
  }

  return (
    <div>
      <Title level={4}>版本升级管理</Title>

      <Card style={{ marginBottom: 16 }}>
        <Space wrap>
          <Button type="primary" icon={<PlayCircleOutlined />} onClick={() => setPlanModalVisible(true)}>
            规划升级路径
          </Button>
          <Button icon={<CheckCircleOutlined />} onClick={() => setCompatModalVisible(true)}>
            兼容性检查
          </Button>
          <Button icon={<SyncOutlined />} onClick={() => setInPlaceModalVisible(true)}>
            原地升级
          </Button>
          <Button icon={<SwapOutlined />} onClick={() => setLogicalModalVisible(true)}>
            逻辑迁移
          </Button>
          <Button icon={<SyncOutlined />} onClick={() => setRollingModalVisible(true)}>
            滚动升级
          </Button>
          <Button icon={<DownloadOutlined />} onClick={handleDownloadReport}>
            下载升级报告
          </Button>
        </Space>
      </Card>

      <Tabs defaultActiveKey="history">
        <TabPane tab={<span><HistoryOutlined /> 升级历史</span>} key="history">
          <Table
            columns={columns}
            dataSource={mockHistory}
            rowKey="id"
            pagination={{ pageSize: 10 }}
          />
        </TabPane>
        <TabPane tab={<span><ExclamationCircleOutlined /> 兼容性警告</span>} key="warnings">
          <Card>
            <p>暂无兼容性警告</p>
          </Card>
        </TabPane>
        <TabPane tab={<span><FileTextOutlined /> 升级报告</span>} key="reports">
          <Card>
            <Space direction="vertical" style={{ width: '100%' }}>
              <Button icon={<DownloadOutlined />} onClick={handleDownloadReport}>
                下载完整报告
              </Button>
              <Divider />
              <Typography.Text>
                最近报告: 2024-01-17 MySQL 5.7.38 升级至 8.0.32
              </Typography.Text>
            </Space>
          </Card>
        </TabPane>
      </Tabs>

      {/* PlanUpgradePath Modal */}
      <Modal
        title="规划升级路径"
        open={planModalVisible}
        onCancel={() => setPlanModalVisible(false)}
        onOk={() => planForm.submit()}
        width={600}
      >
        <Form form={planForm} layout="vertical" onFinish={handlePlanUpgradePath}>
          <Form.Item name="instance_id" label="实例ID" rules={[{ required: true }]}>
            <Input placeholder="请输入实例ID" />
          </Form.Item>
          <Form.Item name="source_version" label="源版本" rules={[{ required: true }]}>
            <Select placeholder="选择源版本">
              <Select.Option value="5.6.51">MySQL 5.6.51</Select.Option>
              <Select.Option value="5.7.38">MySQL 5.7.38</Select.Option>
              <Select.Option value="5.7.42">MySQL 5.7.42</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="target_version" label="目标版本" rules={[{ required: true }]}>
            <Select placeholder="选择目标版本">
              <Select.Option value="5.7.42">MySQL 5.7.42</Select.Option>
              <Select.Option value="8.0.32">MySQL 8.0.32</Select.Option>
              <Select.Option value="8.0.33">MySQL 8.0.33</Select.Option>
              <Select.Option value="8.0.35">MySQL 8.0.35</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="upgrade_strategy" label="升级策略" rules={[{ required: true }]}>
            <Select placeholder="选择升级策略">
              <Select.Option value="in_place">原地升级</Select.Option>
              <Select.Option value="logical">逻辑迁移</Select.Option>
              <Select.Option value="rolling">滚动升级</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="backup_method" label="备份方式">
            <Select placeholder="选择备份方式">
              <Select.Option value="full">全量备份</Select.Option>
              <Select.Option value="incremental">增量备份</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="scheduled_time" label="计划执行时间">
            <DatePicker showTime style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      {/* CheckCompatibility Modal */}
      <Modal
        title="兼容性检查"
        open={compatModalVisible}
        onCancel={() => {
          setCompatModalVisible(false)
          setCompatResult(null)
        }}
        footer={[
          <Button key="cancel" onClick={() => {
            setCompatModalVisible(false)
            setCompatResult(null)
          }}>
            取消
          </Button>,
          <Button key="check" type="primary" onClick={() => compatForm.submit()}>
            开始检查
          </Button>,
        ]}
        width={800}
      >
        <Form form={compatForm} layout="vertical" onFinish={handleCheckCompatibility}>
          <Form.Item name="instance_id" label="实例ID" rules={[{ required: true }]}>
            <Input placeholder="请输入实例ID" />
          </Form.Item>
          <Form.Item name="target_version" label="目标版本" rules={[{ required: true }]}>
            <Select placeholder="选择目标版本">
              <Select.Option value="8.0.32">MySQL 8.0.32</Select.Option>
              <Select.Option value="8.0.33">MySQL 8.0.33</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="check_scope" label="检查范围">
            <Select mode="multiple" placeholder="选择检查项目">
              <Select.Option value="sql_mode">SQL_MODE 兼容性</Select.Option>
              <Select.Option value="deprecated">已废弃特性</Select.Option>
              <Select.Option value="reserved_words">保留字检查</Select.Option>
              <Select.Option value="charset">字符集兼容性</Select.Option>
              <Select.Option value="engine">存储引擎兼容性</Select.Option>
            </Select>
          </Form.Item>
        </Form>

        {compatResult && (
          <div style={{ marginTop: 24 }}>
            <Divider>检查结果</Divider>
            <Card>
              <p><Tag color="success">兼容性检查通过</Tag></p>
              <Title level={5}>警告项 ({compatResult.warnings.length})</Title>
              <ul>
                {compatResult.warnings.map((w: string, i: number) => (
                  <li key={i}><Tag color="warning">{w}</Tag></li>
                ))}
              </ul>
              <Title level={5}>建议</Title>
              <ul>
                {compatResult.recommendations.map((r: string, i: number) => (
                  <li key={i}>{r}</li>
                ))}
              </ul>
            </Card>
          </div>
        )}
      </Modal>

      {/* InPlace Upgrade Modal */}
      <Modal
        title="原地升级"
        open={inPlaceModalVisible}
        onCancel={() => {
          setInPlaceModalVisible(false)
          setUpgradeProgress(0)
          setCurrentStep(0)
        }}
        onOk={() => inPlaceForm.submit()}
        width={700}
      >
        <Form form={inPlaceForm} layout="vertical" onFinish={handleInPlaceUpgrade}>
          <Form.Item name="instance_id" label="实例ID" rules={[{ required: true }]}>
            <Input placeholder="请输入实例ID" />
          </Form.Item>
          <Form.Item name="target_version" label="目标版本" rules={[{ required: true }]}>
            <Select placeholder="选择目标版本">
              <Select.Option value="8.0.32">MySQL 8.0.32</Select.Option>
              <Select.Option value="8.0.33">MySQL 8.0.33</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="backup_path" label="备份路径">
            <Input placeholder="/data/backup/mysql" />
          </Form.Item>
          <Form.Item name="stop_app_timeout" label="停止应用超时时间(秒)">
            <InputNumber min={30} max={600} defaultValue={300} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="skip_backup" label="跳过备份" valuePropName="checked">
            <Select placeholder="选择">
              <Select.Option value={false}>否</Select.Option>
              <Select.Option value={true}>是 (不推荐)</Select.Option>
            </Select>
          </Form.Item>
        </Form>

        {upgradeProgress > 0 && (
          <div style={{ marginTop: 24 }}>
            <Divider>升级进度</Divider>
            {renderUpgradeSteps()}
            <Progress percent={upgradeProgress} style={{ marginTop: 16 }} />
          </div>
        )}
      </Modal>

      {/* Logical Migration Modal */}
      <Modal
        title="逻辑迁移"
        open={logicalModalVisible}
        onCancel={() => {
          setLogicalModalVisible(false)
          setUpgradeProgress(0)
          setCurrentStep(0)
        }}
        onOk={() => logicalForm.submit()}
        width={700}
      >
        <Form form={logicalForm} layout="vertical" onFinish={handleLogicalMigration}>
          <Form.Item name="source_instance_id" label="源实例ID" rules={[{ required: true }]}>
            <Input placeholder="请输入源实例ID" />
          </Form.Item>
          <Form.Item name="target_instance_id" label="目标实例ID" rules={[{ required: true }]}>
            <Input placeholder="请输入目标实例ID" />
          </Form.Item>
          <Form.Item name="target_version" label="目标版本" rules={[{ required: true }]}>
            <Select placeholder="选择目标版本">
              <Select.Option value="8.0.32">MySQL 8.0.32</Select.Option>
              <Select.Option value="8.0.33">MySQL 8.0.33</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="parallel_threads" label="并行线程数">
            <InputNumber min={1} max={16} defaultValue={4} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="batch_size" label="批次大小">
            <InputNumber min={100} max={10000} defaultValue={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="databases" label="迁移数据库">
            <Select mode="tags" placeholder="选择或输入数据库名">
              <Select.Option value="db1">db1</Select.Option>
              <Select.Option value="db2">db2</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="verify_data" label="数据校验" valuePropName="checked">
            <Select defaultValue={true}>
              <Select.Option value={true}>是</Select.Option>
              <Select.Option value={false}>否</Select.Option>
            </Select>
          </Form.Item>
        </Form>

        {upgradeProgress > 0 && (
          <div style={{ marginTop: 24 }}>
            <Divider>迁移进度</Divider>
            {renderUpgradeSteps()}
            <Progress percent={upgradeProgress} style={{ marginTop: 16 }} />
          </div>
        )}
      </Modal>

      {/* Rolling Upgrade Modal */}
      <Modal
        title="滚动升级"
        open={rollingModalVisible}
        onCancel={() => {
          setRollingModalVisible(false)
          setUpgradeProgress(0)
          setCurrentStep(0)
        }}
        onOk={() => rollingForm.submit()}
        width={700}
      >
        <Form form={rollingForm} layout="vertical" onFinish={handleRollingUpgrade}>
          <Form.Item name="cluster_id" label="集群ID" rules={[{ required: true }]}>
            <Input placeholder="请输入集群ID" />
          </Form.Item>
          <Form.Item name="target_version" label="目标版本" rules={[{ required: true }]}>
            <Select placeholder="选择目标版本">
              <Select.Option value="8.0.32">MySQL 8.0.32</Select.Option>
              <Select.Option value="8.0.33">MySQL 8.0.33</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="upgrade_order" label="升级顺序">
            <Select placeholder="选择升级顺序">
              <Select.Option value="replica_first">从节点优先</Select.Option>
              <Select.Option value="primary_first">主节点优先</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="wait_replica_sync" label="等待从节点同步" valuePropName="checked">
            <Select defaultValue={true}>
              <Select.Option value={true}>是</Select.Option>
              <Select.Option value={false}>否</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="health_check_interval" label="健康检查间隔(秒)">
            <InputNumber min={5} max={60} defaultValue={10} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="auto_failback" label="自动回滚">
            <Select defaultValue={true}>
              <Select.Option value={true}>失败时自动回滚</Select.Option>
              <Select.Option value={false}>手动处理</Select.Option>
            </Select>
          </Form.Item>
        </Form>

        {upgradeProgress > 0 && (
          <div style={{ marginTop: 24 }}>
            <Divider>升级进度</Divider>
            {renderUpgradeSteps()}
            <Progress percent={upgradeProgress} style={{ marginTop: 16 }} />
          </div>
        )}
      </Modal>

      {/* Report Modal */}
      <Modal
        title="升级报告"
        open={reportModalVisible}
        onCancel={() => setReportModalVisible(false)}
        footer={[
          <Button key="close" onClick={() => setReportModalVisible(false)}>
            关闭
          </Button>,
          <Button key="download" type="primary" icon={<DownloadOutlined />} onClick={handleDownloadReport}>
            下载报告
          </Button>,
        ]}
        width={800}
      >
        <Card>
          <Typography.Text>
            <pre style={{ whiteSpace: 'pre-wrap' }}>
{`
升级报告详情
==================
实例: MySQL-生产-01
源版本: 5.7.38
目标版本: 8.0.32
升级类型: 原地升级
状态: 成功
开始时间: 2024-01-15 10:00:00
结束时间: 2024-01-15 10:30:00
持续时间: 30分钟

执行步骤:
1. 停止 MySQL 服务 - 成功
2. 备份数据目录 - 成功
3. 替换二进制文件 - 成功
4. 启动 MySQL 服务 - 成功
5. 执行 mysql_upgrade - 成功
6. 验证升级结果 - 成功

性能对比:
- 升级前 TPS: 12,000
- 升级后 TPS: 15,000
- 性能提升: 25%
`}
            </pre>
          </Typography.Text>
        </Card>
      </Modal>
    </div>
  )
}

export default UpgradeManage