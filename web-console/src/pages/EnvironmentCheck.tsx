import React, { useState } from 'react'
import { Card, Steps, Button, Form, Input, Space, Table, Tag, message, Spin } from 'antd'
import { PlayCircleOutlined, DownloadOutlined } from '@ant-design/icons'
import { envCheckApi } from '@/services/api'

interface HostConfig {
  host: string
  port: number
  username: string
  password: string
}

interface CheckResult {
  category: string
  name: string
  status: string
  passed: boolean
  value: string
  suggestion: string
}

const EnvironmentCheck: React.FC = () => {
  const [currentStep, setCurrentStep] = useState(0)
  const [hosts, setHosts] = useState<HostConfig[]>([])
  const [checkId, setCheckId] = useState<string>('')
  const [results, setResults] = useState<CheckResult[]>([])
  const [loading, setLoading] = useState(false)
  const [form] = Form.useForm()

  const handleAddHost = () => {
    const values = form.getFieldsValue()
    if (values.host && values.port && values.username && values.password) {
      setHosts([...hosts, values])
      form.resetFields()
      message.success('主机已添加')
    }
  }

  const handleExecuteCheck = async () => {
    if (hosts.length === 0) {
      message.error('请先添加主机')
      return
    }

    setLoading(true)
    try {
      const response = await envCheckApi.execute(hosts)
      setCheckId(response.data.check_id)
      setResults(response.data.results)
      setCurrentStep(2)
      message.success('环境检测完成')
    } catch (err) {
      message.error('检测失败')
    } finally {
      setLoading(false)
    }
  }

  const handleExport = async (format: string) => {
    try {
      await envCheckApi.export(checkId, format)
      message.success('报告导出成功')
    } catch (err) {
      message.error('导出失败')
    }
  }

  const columns = [
    {
      title: '类别',
      dataIndex: 'category',
      key: 'category',
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: '检测项',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '状态',
      dataIndex: 'passed',
      key: 'passed',
      render: (passed: boolean) => (
        <Tag color={passed ? 'success' : 'error'}>{passed ? '通过' : '失败'}</Tag>
      ),
    },
    {
      title: '值',
      dataIndex: 'value',
      key: 'value',
    },
    {
      title: '建议',
      dataIndex: 'suggestion',
      key: 'suggestion',
      render: (text: string) => text || '-',
    },
  ]

  const hostColumns = [
    {
      title: '主机',
      dataIndex: 'host',
      key: 'host',
    },
    {
      title: '端口',
      dataIndex: 'port',
      key: 'port',
    },
    {
      title: '用户名',
      dataIndex: 'username',
      key: 'username',
    },
  ]

  return (
    <Card title="环境检测">
      <Steps current={currentStep} style={{ marginBottom: 24 }}>
        <Steps.Step title="添加主机" />
        <Steps.Step title="执行检测" />
        <Steps.Step title="查看结果" />
      </Steps>

      {currentStep === 0 && (
        <div>
          <Form form={form} layout="inline" style={{ marginBottom: 16 }}>
            <Form.Item name="host" rules={[{ required: true }]}>
              <Input placeholder="主机地址" />
            </Form.Item>
            <Form.Item name="port" rules={[{ required: true }]} initialValue={3306}>
              <Input type="number" placeholder="端口" style={{ width: 100 }} />
            </Form.Item>
            <Form.Item name="username" rules={[{ required: true }]} initialValue="root">
              <Input placeholder="用户名" />
            </Form.Item>
            <Form.Item name="password" rules={[{ required: true }]}>
              <Input.Password placeholder="密码" />
            </Form.Item>
            <Form.Item>
              <Button onClick={handleAddHost}>添加</Button>
            </Form.Item>
          </Form>

          <Table
            columns={hostColumns}
            dataSource={hosts}
            rowKey="host"
            pagination={false}
            style={{ marginBottom: 16 }}
          />

          <Button
            type="primary"
            disabled={hosts.length === 0}
            onClick={() => setCurrentStep(1)}
          >
            下一步
          </Button>
        </div>
      )}

      {currentStep === 1 && (
        <div style={{ textAlign: 'center', padding: '40px 0' }}>
          <Spin spinning={loading} />
          <p style={{ marginTop: 16 }}>准备执行环境检测...</p>
          <Space>
            <Button onClick={() => setCurrentStep(0)}>返回</Button>
            <Button
              type="primary"
              icon={<PlayCircleOutlined />}
              loading={loading}
              onClick={handleExecuteCheck}
            >
              开始检测
            </Button>
          </Space>
        </div>
      )}

      {currentStep === 2 && (
        <div>
          <Table
            columns={columns}
            dataSource={results}
            rowKey={(record) => `${record.category}-${record.name}`}
          />

          <div style={{ marginTop: 16 }}>
            <Space>
              <Button icon={<DownloadOutlined />} onClick={() => handleExport('json')}>
                导出 JSON
              </Button>
              <Button icon={<DownloadOutlined />} onClick={() => handleExport('pdf')}>
                导出 PDF
              </Button>
            </Space>
          </div>
        </div>
      )}
    </Card>
  )
}

export default EnvironmentCheck