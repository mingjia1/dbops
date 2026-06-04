import React, { useEffect, useState } from 'react'
import {
  Card, Form, Input, InputNumber, Button, Space, Table, Tag, message, Empty, Progress, Row, Col, Statistic, Radio, Alert, Spin,
} from 'antd'
import {
  PlusOutlined, MinusCircleOutlined, PlayCircleOutlined, DownloadOutlined, CheckCircleOutlined, CloseCircleOutlined,
  SettingOutlined, DesktopOutlined, ReloadOutlined,
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { envCheckApi, hostApi, type Host } from '../services/api'

interface CheckItem {
  category: string
  name: string
  status: string
  passed: boolean
  value: string
  suggestion: string
}

interface CheckResult {
  check_id: string
  status: string
  created_at: string
  results: CheckItem[]
}

const EnvironmentCheck: React.FC = () => {
  const [form] = Form.useForm()
  const [submitting, setSubmitting] = useState(false)
  const [result, setResult] = useState<CheckResult | null>(null)
  const [hosts, setHosts] = useState<Host[]>([])
  const [hostsLoading, setHostsLoading] = useState(false)
  const [mode, setMode] = useState<'from-hosts' | 'manual'>('from-hosts')
  const [selectedHosts, setSelectedHosts] = useState<string[]>([])

  const fetchHosts = async () => {
    setHostsLoading(true)
    try {
      const res: any = await hostApi.list(100, 0)
      setHosts(res?.data || [])
    } catch {
      setHosts([])
    } finally {
      setHostsLoading(false)
    }
  }

  useEffect(() => {
    fetchHosts()
  }, [])

  const onFinish = async () => {
    let payload: { hosts: { host: string; port: number; username: string; password: string }[] } = { hosts: [] }

    if (mode === 'from-hosts') {
      if (selectedHosts.length === 0) {
        message.warning('请至少选择一台主机')
        return
      }
      const okHosts = hosts.filter((h) => selectedHosts.includes(h.id) && h.status === 'success')
      if (okHosts.length === 0) {
        message.warning('所选主机均未通过可用性检测, 请先在主机管理中点击"测试连接"')
        return
      }
      payload = {
        hosts: okHosts.map((h) => ({
          host: h.address,
          port: h.ssh_port,
          username: h.ssh_user,
          password: '',
        })),
      }
    } else {
      const values = await form.validateFields()
      if (!values.hosts || values.hosts.length === 0) {
        message.warning('请至少添加一个主机')
        return
      }
      payload = { hosts: values.hosts }
    }

    setSubmitting(true)
    try {
      const res: any = await envCheckApi.execute(payload)
      setResult(res?.data || null)
      message.success('环境检测完成')
    } catch (err: any) {
      message.error(err?.response?.data?.message || '环境检测失败')
    } finally {
      setSubmitting(false)
    }
  }

  const handleExport = async () => {
    if (!result) return
    try {
      const res: any = await envCheckApi.export(result.check_id, 'json')
      const blob = new Blob([JSON.stringify(res?.data || res, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${result.check_id}.json`
      a.click()
      URL.revokeObjectURL(url)
      message.success('导出成功')
    } catch (err: any) {
      message.error(err?.response?.data?.message || '导出失败')
    }
  }

  const columns: ColumnsType<CheckItem> = [
    { title: '类别', dataIndex: 'category', key: 'category', width: 120 },
    { title: '检查项', dataIndex: 'name', key: 'name', width: 200 },
    {
      title: '状态',
      dataIndex: 'passed',
      key: 'passed',
      width: 100,
      render: (passed: boolean) => (
        <Tag color={passed ? 'success' : 'error'} icon={passed ? <CheckCircleOutlined /> : <CloseCircleOutlined />}>
          {passed ? '通过' : '失败'}
        </Tag>
      ),
    },
    { title: '当前值', dataIndex: 'value', key: 'value', width: 200 },
    { title: '建议', dataIndex: 'suggestion', key: 'suggestion' },
  ]

  const total = result?.results.length || 0
  const passed = result?.results.filter((r) => r.passed).length || 0
  const failed = total - passed
  const score = total > 0 ? Math.round((passed / total) * 100) : 0

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title={
          <Space>
            <SettingOutlined />
            <span>环境检测</span>
          </Space>
        }
        extra={
          <Radio.Group
            value={mode}
            onChange={(e) => setMode(e.target.value)}
            optionType="button"
            buttonStyle="solid"
          >
            <Radio.Button value="from-hosts">
              <DesktopOutlined /> 从主机列表
            </Radio.Button>
            <Radio.Button value="manual">
              手动输入
            </Radio.Button>
          </Radio.Group>
        }
      >
        {mode === 'from-hosts' ? (
          <Spin spinning={hostsLoading}>
            {hosts.length === 0 ? (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={
                  <div>
                    <div style={{ marginBottom: 8 }}>暂无主机, 请先在"主机与实例 → 主机管理"中添加</div>
                    <Button type="primary" onClick={() => window.history.back()}>返回主机管理</Button>
                  </div>
                }
              />
            ) : (
              <>
                <Alert
                  type="info"
                  showIcon
                  style={{ marginBottom: 12 }}
                  message="提示"
                  description={
                    <div>
                      <div>• 平台将使用主机管理中已配置的 SSH 用户与凭据, 你无需再次输入。</div>
                      <div>• 只有<span style={{ color: '#3f8600' }}> <b>可用</b> </span>状态的主机可以参与检测, 不可用主机将被跳过。</div>
                      <div>• 检测会 SSH 登录到目标主机执行 MySQL 部署前检查 (磁盘、内核、依赖等)。</div>
                    </div>
                  }
                />
                <Table
                  rowKey="id"
                  size="small"
                  pagination={false}
                  dataSource={hosts}
                  rowSelection={{
                    selectedRowKeys: selectedHosts,
                    onChange: (keys) => setSelectedHosts(keys as string[]),
                    getCheckboxProps: (r: Host) => ({
                      disabled: r.status !== 'success',
                    }),
                  }}
                  columns={[
                    { title: '主机名称', dataIndex: 'name', key: 'name' },
                    {
                      title: '地址',
                      key: 'address',
                      render: (_, r) => `${r.address}:${r.ssh_port}`,
                    },
                    { title: 'SSH 用户', dataIndex: 'ssh_user', key: 'ssh_user' },
                    { title: '操作系统', dataIndex: 'os_type', key: 'os_type', render: (v) => (v || '-').toUpperCase() },
                    {
                      title: '状态',
                      dataIndex: 'status',
                      key: 'status',
                      render: (s: string) => (
                        <Tag color={s === 'success' ? 'success' : s === 'failed' ? 'error' : 'default'}>
                          {s === 'success' ? '可用' : s === 'failed' ? '不可用' : '未检测'}
                        </Tag>
                      ),
                    },
                  ]}
                />
                <div style={{ marginTop: 16, textAlign: 'right' }}>
                  <Space>
                    <span style={{ color: '#8c8c8c' }}>已选 {selectedHosts.length} / {hosts.length} 台</span>
                    <Button icon={<ReloadOutlined />} onClick={fetchHosts}>刷新</Button>
                    <Button
                      type="primary"
                      icon={<PlayCircleOutlined />}
                      onClick={onFinish}
                      loading={submitting}
                      disabled={selectedHosts.length === 0}
                    >
                      启动环境检测
                    </Button>
                  </Space>
                </div>
              </>
            )}
          </Spin>
        ) : (
          <Form
            form={form}
            layout="vertical"
            onFinish={onFinish}
            initialValues={{ hosts: [{ host: '', port: 22, username: 'root', password: '' }] }}
          >
            <Form.List name="hosts">
              {(fields, { add, remove }) => (
                <>
                  {fields.map(({ key, name, ...restField }) => (
                    <Space key={key} align="baseline" style={{ display: 'flex', marginBottom: 8 }}>
                      <Form.Item {...restField} name={[name, 'host']} rules={[{ required: true, message: '主机IP/域名' }]} style={{ width: 220, marginBottom: 0 }}>
                        <Input placeholder="主机IP/域名" />
                      </Form.Item>
                      <Form.Item {...restField} name={[name, 'port']} rules={[{ required: true }]} style={{ width: 120, marginBottom: 0 }}>
                        <InputNumber min={1} max={65535} placeholder="SSH 端口" style={{ width: '100%' }} />
                      </Form.Item>
                      <Form.Item {...restField} name={[name, 'username']} rules={[{ required: true, message: '用户名' }]} style={{ width: 160, marginBottom: 0 }}>
                        <Input placeholder="SSH 用户名" />
                      </Form.Item>
                      <Form.Item {...restField} name={[name, 'password']} rules={[{ required: true, message: '密码' }]} style={{ width: 200, marginBottom: 0 }}>
                        <Input.Password placeholder="SSH 密码" autoComplete="new-password" />
                      </Form.Item>
                      <MinusCircleOutlined onClick={() => remove(name)} style={{ color: '#ff4d4f' }} />
                    </Space>
                  ))}
                  <Form.Item>
                    <Button type="dashed" icon={<PlusOutlined />} onClick={() => add({ host: '', port: 22, username: 'root', password: '' })} block>
                      添加主机
                    </Button>
                  </Form.Item>
                </>
              )}
            </Form.List>
            <Form.Item>
              <Button type="primary" icon={<PlayCircleOutlined />} htmlType="submit" loading={submitting}>
                启动环境检测
              </Button>
            </Form.Item>
          </Form>
        )}
      </Card>

      {result && (
        <>
          <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
            <Col span={6}><Card><Statistic title="检测ID" value={result.check_id} valueStyle={{ fontSize: 14 }} /></Card></Col>
            <Col span={6}><Card><Statistic title="总检查项" value={total} /></Card></Col>
            <Col span={6}><Card><Statistic title="通过" value={passed} valueStyle={{ color: '#3f8600' }} /></Card></Col>
            <Col span={6}><Card><Statistic title="失败" value={failed} valueStyle={{ color: '#cf1322' }} /></Card></Col>
          </Row>

          <Card
            style={{ marginTop: 16 }}
            title={`环境评分: ${score} / 100`}
            extra={
              <Button icon={<DownloadOutlined />} onClick={handleExport}>
                导出报告
              </Button>
            }
          >
            <Progress percent={score} status={score >= 80 ? 'success' : score >= 60 ? 'active' : 'exception'} />
          </Card>

          <Card style={{ marginTop: 16 }} title="检测结果明细">
            <Table
              columns={columns}
              dataSource={result.results.map((r, i) => ({ ...r, key: `${r.category}-${r.name}-${i}` }))}
              pagination={false}
              size="small"
            />
          </Card>
        </>
      )}

      {!result && (
        <Card style={{ marginTop: 16 }}>
          <Empty description="请选择主机并启动环境检测" />
        </Card>
      )}
    </div>
  )
}

export default EnvironmentCheck
