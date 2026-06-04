import React, { useState } from 'react'
import {
  Card, Form, Input, InputNumber, Button, Space, Table, Tag, message, Empty, Progress, Row, Col, Statistic,
} from 'antd'
import { PlusOutlined, MinusCircleOutlined, PlayCircleOutlined, DownloadOutlined, CheckCircleOutlined, CloseCircleOutlined, SettingOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { envCheckApi } from '../services/api'

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

  const onFinish = async (values: any) => {
    if (!values.hosts || values.hosts.length === 0) {
      message.warning('请至少添加一个主机')
      return
    }
    setSubmitting(true)
    try {
      const res: any = await envCheckApi.execute({ hosts: values.hosts })
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
      >
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
          <Empty description="请填写主机信息并启动环境检测" />
        </Card>
      )}
    </div>
  )
}

export default EnvironmentCheck
