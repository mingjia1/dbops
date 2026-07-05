import React, { useEffect, useState } from 'react'
import {
  Button,
  Card,
  Col,
  Collapse,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Progress,
  Radio,
  Row,
  Space,
  Spin,
  Statistic,
  Table,
  Tag,
  message,
} from 'antd'
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  DesktopOutlined,
  DownloadOutlined,
  MinusCircleOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
  SettingOutlined,
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { envCheckApi, hostApi, type Host } from '../services/api'

interface CheckItem {
  host?: string
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

type Mode = 'from-hosts' | 'manual'

const EnvironmentCheck: React.FC = () => {
  const [form] = Form.useForm()
  const [submitting, setSubmitting] = useState(false)
  const [result, setResult] = useState<CheckResult | null>(null)
  const [hosts, setHosts] = useState<Host[]>([])
  const [hostsLoading, setHostsLoading] = useState(false)
  const [mode, setMode] = useState<Mode>('from-hosts')
  const [selectedHosts, setSelectedHosts] = useState<string[]>([])

  const fetchHosts = async () => {
    setHostsLoading(true)
    try {
      const res: any = await hostApi.list(1000, 0)
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

  const errorMessage = (err: any, fallback: string) =>
    err?.response?.data?.message || err?.message || fallback

  const onFinish = async () => {
    let payload: {
      hosts?: { host: string; port: number; username: string; password: string }[]
      host_ids?: string[]
    } = {}

    if (mode === 'from-hosts') {
      if (selectedHosts.length === 0) {
        message.warning('请至少选择一台主机')
        return
      }
      payload = { host_ids: selectedHosts }
    } else {
      const values = await form.validateFields()
      if (!values.hosts || values.hosts.length === 0) {
        message.warning('请至少添加一台主机')
        return
      }
      payload = { hosts: values.hosts }
    }

    setSubmitting(true)
    try {
      const res: any = await envCheckApi.execute(payload)
      const data = res?.data || null
      setResult(data)
      // Update host status from check results
      if (data?.results) {
        const hostStatus: Record<string, string> = {}
        for (const item of data.results) {
          if (item.host) {
            if (!hostStatus[item.host]) hostStatus[item.host] = 'success'
            // Only network and agent categories determine host pass/fail
            if ((item.category === 'network' || item.category === 'agent') && !item.passed) {
              hostStatus[item.host] = 'failed'
            }
          }
        }
        setHosts(prev => prev.map(h => {
          const status = hostStatus[h.address]
          return status ? { ...h, status } : h
        }))
      }
      // Use backend overall status instead of counting all failed items
      if (data?.status === 'success') {
        message.success('环境检查通过')
      } else {
        const criticalFailed = (data?.results || []).filter(
          (item: CheckItem) => (item.category === 'network' || item.category === 'agent') && !item.passed
        )
        if (criticalFailed.length > 0) {
          Modal.warning({
            title: '环境检查完成',
            content: `${criticalFailed.length} 项关键检查未通过（网络/代理）`,
          })
        } else {
          message.success('环境检查通过')
        }
      }
    } catch (err: any) {
      setResult(null)
      if (mode === 'from-hosts') {
        await fetchHosts()
      }
      message.error(errorMessage(err, '环境检查失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const handleExport = async () => {
    if (!result) return
    try {
      const res: any = await envCheckApi.export(result.check_id, 'json')
      const data = res?.data || res
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `${result.check_id}.json`
      link.click()
      URL.revokeObjectURL(url)
      message.success('导出成功')
    } catch {
      message.error('导出失败')
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
    { title: '当前值', dataIndex: 'value', key: 'value', width: 240 },
    { title: '建议', dataIndex: 'suggestion', key: 'suggestion' },
  ]

  const total = result?.results.length || 0
  const passed = result?.results.filter((item) => item.passed).length || 0
  const failed = total - passed
  const score = total > 0 ? Math.round((passed / total) * 100) : 0
  const groupedResults = result?.results.reduce<Record<string, CheckItem[]>>((acc, item) => {
    const host = item.host || 'unknown'
    if (!acc[host]) acc[host] = []
    acc[host].push(item)
    return acc
  }, {}) || {}
  const hostPanels = Object.entries(groupedResults).map(([host, items]) => {
    const abnormalCount = items.filter((item) => !item.passed).length
    return {
      key: host,
      label: (
        <Space>
          <span>{host}</span>
          <Tag color={abnormalCount > 0 ? 'error' : 'success'}>异常 {abnormalCount}</Tag>
        </Space>
      ),
      children: (
        <Table
          columns={columns}
          dataSource={items.map((item, index) => ({ ...item, key: `${host}-${item.category}-${item.name}-${index}` }))}
          pagination={false}
          size="small"
        />
      ),
    }
  })

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={<Space><SettingOutlined /><span>环境检查</span></Space>}
        extra={
          <Space>
            {mode === 'from-hosts' && hosts.length > 0 && (
              <>
                <Button icon={<ReloadOutlined />} onClick={fetchHosts}>刷新</Button>
                <Button type="primary" icon={<PlayCircleOutlined />} onClick={onFinish} loading={submitting} disabled={selectedHosts.length === 0}>
                  启动环境检查
                </Button>
              </>
            )}
            <Radio.Group value={mode} onChange={(event) => setMode(event.target.value)} optionType="button" buttonStyle="solid">
              <Radio.Button value="from-hosts"><DesktopOutlined /> 从主机列表</Radio.Button>
              <Radio.Button value="manual">手动输入</Radio.Button>
            </Radio.Group>
          </Space>
        }
      >
        {mode === 'from-hosts' ? (
          <Spin spinning={hostsLoading}>
            {hosts.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无主机，请先在主机管理中添加" />
            ) : (
              <>
                <div style={{ marginBottom: 12, display: 'flex', alignItems: 'center', gap: 12 }}>
                  <span style={{ color: '#8c8c8c' }}>已选 {selectedHosts.length} / {hosts.length} 台</span>
                </div>
                <Table
                  rowKey="id"
                  size="small"
                  pagination={false}
                  dataSource={hosts}
                  rowSelection={{
                    selectedRowKeys: selectedHosts,
                    onChange: (keys) => setSelectedHosts(keys as string[]),
                  }}
                  columns={[
                    { title: '主机名称', dataIndex: 'name', key: 'name' },
                    { title: '地址', key: 'address', render: (_, host) => `${host.address}:${host.ssh_port}` },
                    { title: 'SSH 用户', dataIndex: 'ssh_user', key: 'ssh_user' },
                    { title: '操作系统', dataIndex: 'os_type', key: 'os_type', render: (value) => (value || '-').toUpperCase() },
                    {
                      title: '状态',
                      dataIndex: 'status',
                      key: 'status',
                      render: (status: string) => (
                        <Tag color={status === 'success' ? 'success' : status === 'failed' ? 'error' : 'default'}>
                          {status === 'success' ? '可用' : status === 'failed' ? '不可用' : '未检测'}
                        </Tag>
                      ),
                    },
                  ]}
                />
              </>
            )}
          </Spin>
        ) : (
          <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ hosts: [{ host: '', port: 22, username: 'root', password: '' }] }}>
            <Form.List name="hosts">
              {(fields, { add, remove }) => (
                <>
                  {fields.map(({ key, name, ...restField }) => (
                    <Space key={key} align="baseline" style={{ display: 'flex', marginBottom: 8 }}>
                      <Form.Item {...restField} name={[name, 'host']} rules={[{ required: true, message: '请输入主机 IP 或域名' }]} style={{ width: 220, marginBottom: 0 }}>
                        <Input placeholder="主机 IP / 域名" />
                      </Form.Item>
                      <Form.Item {...restField} name={[name, 'port']} rules={[{ required: true, message: '请输入 SSH 端口' }]} style={{ width: 120, marginBottom: 0 }}>
                        <InputNumber min={1} max={65535} placeholder="SSH 端口" style={{ width: '100%' }} />
                      </Form.Item>
                      <Form.Item {...restField} name={[name, 'username']} rules={[{ required: true, message: '请输入 SSH 用户名' }]} style={{ width: 160, marginBottom: 0 }}>
                        <Input placeholder="SSH 用户名" />
                      </Form.Item>
                      <Form.Item {...restField} name={[name, 'password']} rules={[{ required: true, message: '请输入 SSH 密码' }]} style={{ width: 220, marginBottom: 0 }}>
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
              <Button type="primary" icon={<PlayCircleOutlined />} htmlType="submit" loading={submitting}>启动环境检查</Button>
            </Form.Item>
          </Form>
        )}
      </Card>

      {result && (
        <>
          <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
            <Col span={6}><Card><Statistic title="检查 ID" value={result.check_id} valueStyle={{ fontSize: 14 }} /></Card></Col>
            <Col span={6}><Card><Statistic title="总检查项" value={total} /></Card></Col>
            <Col span={6}><Card><Statistic title="通过" value={passed} valueStyle={{ color: '#3f8600' }} /></Card></Col>
            <Col span={6}><Card><Statistic title="失败" value={failed} valueStyle={{ color: '#cf1322' }} /></Card></Col>
          </Row>

          <Card style={{ marginTop: 16 }} title={`环境评分：${score} / 100`} extra={<Button icon={<DownloadOutlined />} onClick={handleExport}>导出报告</Button>}>
            <Progress percent={score} status={score >= 80 ? 'success' : score >= 60 ? 'active' : 'exception'} />
          </Card>

          <Card style={{ marginTop: 16 }} title="检查结果明细">
            <Collapse items={hostPanels} />
          </Card>
        </>
      )}

      {!result && <Card style={{ marginTop: 16 }}><Empty description="请选择主机并启动环境检查" /></Card>}
    </div>
  )
}

export default EnvironmentCheck
