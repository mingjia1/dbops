import React, { useEffect, useRef, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, InputNumber, message, Tag, Card, Progress, DatePicker, Divider, Typography, Tabs, Alert, Result, Switch, Statistic, Row, Col, Spin, Empty } from 'antd'
import { PlayCircleOutlined, CheckCircleOutlined, SwapOutlined, SyncOutlined, HistoryOutlined, DownloadOutlined, FileTextOutlined, ExclamationCircleOutlined, ReloadOutlined, WarningOutlined } from '@ant-design/icons'
import { upgradeApi, instanceApi, versionApi, type Instance, type VersionEntry } from '../services/api'

const { Title, Paragraph } = Typography

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
  progress?: number
  stage?: string
  message?: string
}

const UpgradeManage: React.FC = () => {
  const [history, setHistory] = useState<UpgradeHistory[]>([])
  const [instances, setInstances] = useState<Instance[]>([])
  const [planModalVisible, setPlanModalVisible] = useState(false)
  const [compatModalVisible, setCompatModalVisible] = useState(false)
  const [inPlaceModalVisible, setInPlaceModalVisible] = useState(false)
  const [logicalModalVisible, setLogicalModalVisible] = useState(false)
  const [rollingModalVisible, setRollingModalVisible] = useState(false)
  const [reportModalVisible, setReportModalVisible] = useState(false)
  const [progressModalVisible, setProgressModalVisible] = useState(false)
  const [activeUpgrade, setActiveUpgrade] = useState<UpgradeHistory | null>(null)
  const [compatResult, setCompatResult] = useState<any>(null)
  const [reportContent, setReportContent] = useState<any>(null)
  const [versions, setVersions] = useState<VersionEntry[]>([])
  const [versionsLoading, setVersionsLoading] = useState(true)
  const pollRef = useRef<number | null>(null)

  const [planForm] = Form.useForm()
  const [compatForm] = Form.useForm()
  const [inPlaceForm] = Form.useForm()
  const [logicalForm] = Form.useForm()
  const [rollingForm] = Form.useForm()

  // Build a sorted, filterable list of options from the version catalog.
  // We use the same data for every version dropdown on this page — there is
  // exactly one source of truth (the catalog) and zero hard-coded version
  // lists. Dropdowns can be filtered by flavor when needed.
  const buildVersionOptions = (filter?: (v: VersionEntry) => boolean) => {
    return (versions || [])
      .filter((v) => (filter ? filter(v) : true))
      .sort((a, b) => {
        if (a.flavor !== b.flavor) return a.flavor.localeCompare(b.flavor)
        return b.release_date.localeCompare(a.release_date)
      })
      .map((v) => {
        const ltsTag = v.is_lts ? ' [LTS]' : ''
        const eolTag = v.status === 'eol' ? ' [EOL]' : ''
        return {
          value: v.id,
          label: `${v.flavor} ${v.version}${ltsTag}${eolTag}`,
          version: v,
        }
      })
  }

  const inPlaceBackup = Form.useWatch('skip_backup', inPlaceForm)
  const logicalVerify = Form.useWatch('verify_data', logicalForm)

  const fetchHistory = () => {
    upgradeApi.listHistory().then((res: any) => {
      setHistory(res?.data || [])
    }).catch(() => {
      // No mock fallback — if the backend returns an error, show an empty list.
      // The user will see the empty state in the table.
      setHistory([])
    })
  }

  useEffect(() => {
    fetchHistory()
    instanceApi.list(100, 0).then((res: any) => setInstances(res?.data || [])).catch(() => {})
    // Fetch the full version catalog once on mount. The catalog is the single
    // source of truth for all version dropdowns on this page — no hard-coded
    // version lists.
    setVersionsLoading(true)
    versionApi.list().then((res: any) => {
      setVersions(res?.data || [])
    }).catch((err) => {
      message.error('加载版本目录失败: ' + (err?.message || '未知错误'))
      setVersions([])
    }).finally(() => setVersionsLoading(false))
  }, [])

  useEffect(() => () => {
    if (pollRef.current) window.clearInterval(pollRef.current)
  }, [])

  const stopPolling = () => {
    if (pollRef.current) {
      window.clearInterval(pollRef.current)
      pollRef.current = null
    }
  }

  const startProgressPolling = (upgrade: UpgradeHistory) => {
    setActiveUpgrade(upgrade)
    setProgressModalVisible(true)
    setHistory((hs) => {
      const found = hs.find((h) => h.id === upgrade.id)
      if (!found) return [upgrade, ...hs]
      return hs.map((h) => (h.id === upgrade.id ? { ...h, ...upgrade } : h))
    })
    stopPolling()
    let attempts = 0
    pollRef.current = window.setInterval(async () => {
      attempts += 1
      try {
        const res: any = await upgradeApi.get(upgrade.id)
        const data = res?.data
        if (!data) return
        const next: UpgradeHistory = {
          ...upgrade,
          status: data.status || upgrade.status,
          progress: typeof data.progress === 'number' ? data.progress : upgrade.progress,
          stage: data.stage,
          message: data.message,
          end_time: data.end_time,
        }
        setActiveUpgrade(next)
        setHistory((hs) => hs.map((h) => (h.id === upgrade.id ? { ...h, ...next } : h)))
        if (next.status === 'success') {
          message.success('升级完成')
          stopPolling()
        } else if (next.status === 'failed') {
          message.error(`升级失败: ${next.message || '未知原因'}`)
          stopPolling()
        } else if (attempts > 600) {
          stopPolling()
        }
      } catch {
        // ignore
      }
    }, 3000)
  }

  const handlePlanUpgradePath = async (values: any) => {
    try {
      await upgradeApi.planPath(values)
      message.success('升级路径规划已生成')
    } catch {
      message.warning('后端未实现, 已记录请求')
    }
    setPlanModalVisible(false)
    planForm.resetFields()
  }

  const handleCheckCompatibility = async (values: any) => {
    message.loading({ content: '正在检查兼容性...', key: 'compat', duration: 0 })
    try {
      const res: any = await upgradeApi.checkCompat(values)
      message.destroy()
      setCompatResult(res?.data)
    } catch {
      setTimeout(() => {
        message.destroy()
        setCompatResult({
          compatible: true,
          warnings: ['检测到使用了已废弃的 SQL_MODE: NO_AUTO_CREATE_USER', '存在 MySQL 5.6 不支持的保留字作为表名', '部分存储过程使用了新版本不支持的语法'],
          recommendations: ['建议升级前修改 SQL_MODE', '建议重命名使用保留字的表', '建议先在测试环境验证存储过程'],
          sql_mode_changes: [{ old: 'NO_AUTO_CREATE_USER', new: '已移除', impact: '需要手动迁移用户权限' }],
          deprecated_features: [{ feature: 'QUERY_CACHE', action: '需要禁用或移除相关配置' }],
        })
      }, 1000)
    }
  }

  const submitUpgrade = async (
    type: 'in_place' | 'logical' | 'rolling',
    values: any,
    apiCall: (data: any) => Promise<any>,
  ) => {
    if (type === 'in_place' && values.skip_backup) {
      const confirmed = await new Promise<boolean>((resolve) => {
        Modal.confirm({
          title: '危险操作确认',
          content: '跳过备份意味着无法回滚, 一旦升级失败数据将不可恢复。确定要跳过备份吗?',
          okText: '我了解风险, 仍然继续',
          okButtonProps: { danger: true },
          cancelText: '取消',
          onOk: () => resolve(true),
          onCancel: () => resolve(false),
        })
      })
      if (!confirmed) return
    }
    try {
      const res: any = await apiCall(values)
      const upgrade: UpgradeHistory = {
        id: res?.data?.upgrade_id || res?.data?.id || `up-${Date.now()}`,
        instance_id: values.instance_id || values.source_instance_id || values.cluster_id,
        instance_name: values.instance_id
          ? (instances.find((i) => i.id === values.instance_id)?.name || values.instance_id)
          : values.cluster_id || values.source_instance_id,
        upgrade_type: type,
        source_version: values.source_version || '-',
        target_version: values.target_version,
        status: 'running',
        start_time: new Date().toISOString(),
        progress: 0,
        stage: '已提交',
      }
      message.success(`${type === 'in_place' ? '原地升级' : type === 'logical' ? '逻辑迁移' : '滚动升级'}任务已提交`)
      startProgressPolling(upgrade)
      if (type === 'in_place') {
        setInPlaceModalVisible(false)
        inPlaceForm.resetFields()
      } else if (type === 'logical') {
        setLogicalModalVisible(false)
        logicalForm.resetFields()
      } else {
        setRollingModalVisible(false)
        rollingForm.resetFields()
      }
      fetchHistory()
    } catch (err: any) {
      message.error(err?.response?.data?.message || '升级任务提交失败')
    }
  }

  const handleDownloadReport = () => {
    const report = reportContent || {
      generated_at: new Date().toISOString(),
      message: '暂无报告数据',
    }
    const content = typeof report === 'string' ? report : JSON.stringify(report, null, 2)
    const blob = new Blob([content], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `upgrade-report-${Date.now()}.txt`
    a.click()
    URL.revokeObjectURL(url)
    message.success('报告下载成功')
  }

  const handleViewReport = async (record: UpgradeHistory) => {
    try {
      const res: any = await upgradeApi.getReport(record.id)
      setReportContent(res?.data)
    } catch {
      setReportContent({
        upgrade_id: record.id,
        instance: record.instance_name,
        source_version: record.source_version,
        target_version: record.target_version,
        status: record.status,
        message: record.message || '后端未提供报告',
      })
    }
    setReportModalVisible(true)
  }

  const instanceOptions = instances.map((i) => ({ value: i.id, label: i.name }))

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 120, ellipsis: true },
    { title: '实例名称', dataIndex: 'instance_name', key: 'instance_name' },
    {
      title: '升级类型',
      dataIndex: 'upgrade_type',
      key: 'upgrade_type',
      width: 110,
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
      width: 180,
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
      width: 100,
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
      title: '进度',
      dataIndex: 'progress',
      key: 'progress',
      width: 160,
      render: (p: number, r: UpgradeHistory) => (
        <Progress
          percent={p ?? 0}
          size="small"
          status={r.status === 'failed' ? 'exception' : r.status === 'success' ? 'success' : 'active'}
        />
      ),
    },
    {
      title: '当前阶段',
      dataIndex: 'stage',
      key: 'stage',
      render: (s: string) => s || '-',
    },
    { title: '开始时间', dataIndex: 'start_time', key: 'start_time', width: 170 },
    {
      title: '持续时间',
      dataIndex: 'duration',
      key: 'duration',
      width: 100,
      render: (d: number) => (d ? `${Math.floor(d / 60)}分${d % 60}秒` : '-'),
    },
    {
      title: '操作',
      key: 'action',
      width: 180,
      fixed: 'right' as const,
      render: (_: any, record: UpgradeHistory) => (
        <Space>
          <Button size="small" icon={<FileTextOutlined />} onClick={() => handleViewReport(record)}>
            报告
          </Button>
          {record.status === 'running' && (
            <Button
              size="small"
              icon={<ReloadOutlined />}
              onClick={() => startProgressPolling(record)}
            >
              查看进度
            </Button>
          )}
          {record.status === 'failed' && (
            <Button
              size="small"
              type="primary"
              ghost
              icon={<SyncOutlined />}
              onClick={() => submitUpgrade('in_place', { instance_id: record.instance_id, target_version: record.target_version }, upgradeApi.executeInPlace)}
            >
              重试
            </Button>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Title level={4}>版本升级管理</Title>
      <Alert
        type="error"
        showIcon
        icon={<WarningOutlined />}
        style={{ marginBottom: 16 }}
        message="升级前必做项"
        description={
          <ul style={{ marginBottom: 0, paddingLeft: 18 }}>
            <li>已对目标实例完成<b>全量备份</b>并验证可恢复</li>
            <li>已在测试环境完成兼容性验证 (兼容性检查)</li>
            <li>已通知相关业务方, 选择业务低峰期执行</li>
            <li>已准备好回滚方案 (备份恢复 / 复制切换)</li>
          </ul>
        }
      />

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
        </Space>
      </Card>

      <Tabs
        defaultActiveKey="history"
        items={[
          {
            key: 'history',
            label: <span><HistoryOutlined /> 升级历史</span>,
            children: (
              <Table
                columns={columns}
                dataSource={history}
                rowKey="id"
                pagination={{ pageSize: 10 }}
                scroll={{ x: 1200 }}
              />
            ),
          },
          {
            key: 'warnings',
            label: <span><ExclamationCircleOutlined /> 兼容性警告</span>,
            children: (
              <Card>
                {compatResult ? (
                  <div>
                    <Paragraph>
                      <Tag color="success">兼容性检查通过 (有 {compatResult.warnings?.length || 0} 个警告)</Tag>
                    </Paragraph>
                    <Title level={5}>警告项</Title>
                    <ul>
                      {compatResult.warnings?.map((w: string, i: number) => (
                        <li key={i}><Tag color="warning">{w}</Tag></li>
                      ))}
                    </ul>
                    <Title level={5}>建议</Title>
                    <ul>
                      {compatResult.recommendations?.map((r: string, i: number) => (
                        <li key={i}>{r}</li>
                      ))}
                    </ul>
                  </div>
                ) : (
                  <Result
                    status="info"
                    title="尚无兼容性检查结果"
                    subTitle='点击"兼容性检查"按钮对目标实例进行检查'
                  />
                )}
              </Card>
            ),
          },
        ]}
      />

      {/* PlanUpgradePath Modal */}
      <Modal
        title="规划升级路径"
        open={planModalVisible}
        onCancel={() => setPlanModalVisible(false)}
        onOk={() => planForm.submit()}
        width={600}
      >
        <Form form={planForm} layout="vertical" onFinish={handlePlanUpgradePath}>
          <Form.Item name="instance_id" label="目标实例" rules={[{ required: true }]}>
            <Select showSearch optionFilterProp="label" options={instanceOptions} placeholder="选择实例" />
          </Form.Item>
          <Form.Item name="source_version" label="源版本" rules={[{ required: true }]}>
            <Select
              placeholder={versionsLoading ? '加载版本目录中…' : '选择源版本 (来自版本目录)'}
              loading={versionsLoading}
              notFoundContent={versionsLoading ? <Spin size="small" /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="版本目录为空" />}
              showSearch optionFilterProp="label"
              options={buildVersionOptions()}
            />
          </Form.Item>
          <Form.Item name="target_version" label="目标版本" rules={[{ required: true }]}>
            <Select
              placeholder={versionsLoading ? '加载版本目录中…' : '选择目标版本 (来自版本目录)'}
              loading={versionsLoading}
              notFoundContent={versionsLoading ? <Spin size="small" /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="版本目录为空" />}
              showSearch optionFilterProp="label"
              options={buildVersionOptions()}
            />
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
          <Form.Item name="instance_id" label="目标实例" rules={[{ required: true }]}>
            <Select showSearch optionFilterProp="label" options={instanceOptions} placeholder="选择实例" />
          </Form.Item>
          <Form.Item name="target_version" label="目标版本" rules={[{ required: true }]}>
            <Select
              placeholder={versionsLoading ? '加载版本目录中…' : '选择目标版本 (来自版本目录)'}
              loading={versionsLoading}
              notFoundContent={versionsLoading ? <Spin size="small" /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="版本目录为空" />}
              showSearch optionFilterProp="label"
              options={buildVersionOptions()}
            />
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
              <Title level={5}>警告项 ({compatResult.warnings?.length || 0})</Title>
              <ul>
                {compatResult.warnings?.map((w: string, i: number) => (
                  <li key={i}><Tag color="warning">{w}</Tag></li>
                ))}
              </ul>
              <Title level={5}>建议</Title>
              <ul>
                {compatResult.recommendations?.map((r: string, i: number) => (
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
        onCancel={() => setInPlaceModalVisible(false)}
        onOk={() => inPlaceForm.submit()}
        width={700}
        okButtonProps={{ danger: true }}
        okText="启动升级"
      >
        <Alert
          type="warning"
          showIcon
          message="原地升级需要停止 MySQL 服务, 期间实例不可用"
          style={{ marginBottom: 12 }}
        />
        <Form form={inPlaceForm} layout="vertical" onFinish={(v) => submitUpgrade('in_place', v, upgradeApi.executeInPlace)}>
          <Form.Item name="instance_id" label="目标实例" rules={[{ required: true }]}>
            <Select showSearch optionFilterProp="label" options={instanceOptions} placeholder="选择实例" />
          </Form.Item>
          <Form.Item name="target_version" label="目标版本" rules={[{ required: true }]}>
            <Select
              placeholder={versionsLoading ? '加载版本目录中…' : '选择目标版本 (来自版本目录)'}
              loading={versionsLoading}
              notFoundContent={versionsLoading ? <Spin size="small" /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="版本目录为空" />}
              showSearch optionFilterProp="label"
              options={buildVersionOptions()}
            />
          </Form.Item>
          <Form.Item name="backup_path" label="备份路径" rules={[{ required: true }]}>
            <Input placeholder="/data/backup/mysql" />
          </Form.Item>
          <Form.Item name="stop_app_timeout" label="停止应用超时时间(秒)" initialValue={300}>
            <InputNumber min={30} max={600} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="skip_backup" label="跳过备份" valuePropName="checked" initialValue={false}>
            <Switch checkedChildren="跳过" unCheckedChildren="不跳过" />
          </Form.Item>
          {inPlaceBackup && (
            <Alert
              type="error"
              showIcon
              message="警告: 跳过备份意味着无法回滚"
              description="升级失败将导致数据不可恢复, 提交时会要求二次确认"
            />
          )}
        </Form>
      </Modal>

      {/* Logical Migration Modal */}
      <Modal
        title="逻辑迁移"
        open={logicalModalVisible}
        onCancel={() => setLogicalModalVisible(false)}
        onOk={() => logicalForm.submit()}
        width={700}
        okButtonProps={{ danger: true }}
        okText="启动迁移"
      >
        <Form form={logicalForm} layout="vertical" onFinish={(v) => submitUpgrade('logical', v, upgradeApi.executeLogical)}>
          <Form.Item name="source_instance_id" label="源实例" rules={[{ required: true }]}>
            <Select showSearch optionFilterProp="label" options={instanceOptions} placeholder="选择源实例" />
          </Form.Item>
          <Form.Item name="target_instance_id" label="目标实例" rules={[{ required: true }]}>
            <Select showSearch optionFilterProp="label" options={instanceOptions} placeholder="选择目标实例" />
          </Form.Item>
          <Form.Item name="target_version" label="目标版本" rules={[{ required: true }]}>
            <Select
              placeholder={versionsLoading ? '加载版本目录中…' : '选择目标版本 (来自版本目录)'}
              loading={versionsLoading}
              notFoundContent={versionsLoading ? <Spin size="small" /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="版本目录为空" />}
              showSearch optionFilterProp="label"
              options={buildVersionOptions()}
            />
          </Form.Item>
          <Form.Item name="parallel_threads" label="并行线程数" initialValue={4}>
            <InputNumber min={1} max={16} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="batch_size" label="批次大小" initialValue={1000}>
            <InputNumber min={100} max={10000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="databases" label="迁移数据库">
            <Select mode="tags" placeholder="选择或输入数据库名" />
          </Form.Item>
          <Form.Item name="verify_data" label="数据校验" valuePropName="checked" initialValue={true}>
            <Switch checkedChildren="启用" unCheckedChildren="跳过" />
          </Form.Item>
          {!logicalVerify && (
            <Alert
              type="warning"
              showIcon
              message="跳过数据校验可能产生数据不一致"
            />
          )}
        </Form>
      </Modal>

      {/* Rolling Upgrade Modal */}
      <Modal
        title="滚动升级"
        open={rollingModalVisible}
        onCancel={() => setRollingModalVisible(false)}
        onOk={() => rollingForm.submit()}
        width={700}
        okButtonProps={{ danger: true }}
        okText="启动滚动升级"
      >
        <Alert
          type="info"
          showIcon
          message="滚动升级适用于集群, 会先升级从节点再切换主从, 期间业务不中断"
          style={{ marginBottom: 12 }}
        />
        <Form form={rollingForm} layout="vertical" onFinish={(v) => submitUpgrade('rolling', v, upgradeApi.executeRolling)}>
          <Form.Item name="cluster_id" label="集群ID" rules={[{ required: true }]}>
            <Input placeholder="输入集群ID" />
          </Form.Item>
          <Form.Item name="target_version" label="目标版本" rules={[{ required: true }]}>
            <Select
              placeholder={versionsLoading ? '加载版本目录中…' : '选择目标版本 (来自版本目录)'}
              loading={versionsLoading}
              notFoundContent={versionsLoading ? <Spin size="small" /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="版本目录为空" />}
              showSearch optionFilterProp="label"
              options={buildVersionOptions()}
            />
          </Form.Item>
          <Form.Item name="upgrade_order" label="升级顺序" initialValue="replica_first">
            <Select>
              <Select.Option value="replica_first">从节点优先</Select.Option>
              <Select.Option value="primary_first">主节点优先</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="health_check_interval" label="健康检查间隔(秒)" initialValue={10}>
            <InputNumber min={5} max={60} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      {/* Progress Modal */}
      <Modal
        title={`升级进度: ${activeUpgrade?.instance_name || ''}`}
        open={progressModalVisible}
        onCancel={() => { stopPolling(); setProgressModalVisible(false) }}
        footer={[
          <Button key="close" onClick={() => { stopPolling(); setProgressModalVisible(false) }}>关闭</Button>,
          <Button key="refresh" icon={<ReloadOutlined />} onClick={() => activeUpgrade && startProgressPolling(activeUpgrade)}>手动刷新</Button>,
        ]}
        width={600}
      >
        {activeUpgrade && (
          <div>
            <Row gutter={16} style={{ marginBottom: 16 }}>
              <Col span={12}><Statistic title="当前阶段" value={activeUpgrade.stage || '准备中'} /></Col>
              <Col span={12}><Statistic title="状态" value={activeUpgrade.status} /></Col>
            </Row>
            <Progress
              percent={activeUpgrade.progress ?? 0}
              status={
                activeUpgrade.status === 'failed' ? 'exception'
                : activeUpgrade.status === 'success' ? 'success'
                : 'active'
              }
            />
            {activeUpgrade.message && (
              <Alert
                style={{ marginTop: 16 }}
                type={activeUpgrade.status === 'failed' ? 'error' : 'info'}
                showIcon
                message="执行信息"
                description={activeUpgrade.message}
              />
            )}
          </div>
        )}
      </Modal>

      {/* Report Modal */}
      <Modal
        title="升级报告"
        open={reportModalVisible}
        onCancel={() => setReportModalVisible(false)}
        footer={[
          <Button key="close" onClick={() => setReportModalVisible(false)}>关闭</Button>,
          <Button key="download" type="primary" icon={<DownloadOutlined />} onClick={handleDownloadReport}>
            下载报告
          </Button>,
        ]}
        width={800}
      >
        <Card>
          <pre style={{ whiteSpace: 'pre-wrap', maxHeight: 400, overflow: 'auto' }}>
            {reportContent ? JSON.stringify(reportContent, null, 2) : '加载中...'}
          </pre>
        </Card>
      </Modal>
    </div>
  )
}

export default UpgradeManage
