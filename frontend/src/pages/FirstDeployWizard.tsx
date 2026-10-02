import React, { useEffect, useState } from 'react'
import { Button, Form, Input, InputNumber, message, Modal, Steps, Table, Tag } from 'antd'
import { PlusOutlined, RocketOutlined, ScanOutlined, ThunderboltOutlined } from '@ant-design/icons'
import { hostApi, instanceApi, type Host, type ScannedInstance } from '../services/api'

const STORAGE_KEY = 'dbops_first_deploy_dismissed'

interface HostForm {
  name: string
  address: string
  ssh_port: number
  ssh_user: string
  ssh_password: string
  agent_port: number
}

interface InstanceForm {
  name: string
  host: string
  port: number
  username: string
  password: string
  basedir?: string
  datadir?: string
  os_user?: string
}

const FirstDeployWizard: React.FC = () => {
  const [visible, setVisible] = useState(false)
  const [currentStep, setCurrentStep] = useState(0)
  const [hosts, setHosts] = useState<Host[]>([])
  const [selectedHost, setSelectedHost] = useState<Host | null>(null)
  const [scanning, setScanning] = useState(false)
  const [scannedInstances, setScannedInstances] = useState<ScannedInstance[]>([])
  const [selectedInstances, setSelectedInstances] = useState<ScannedInstance[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [hostForm] = Form.useForm<HostForm>()
  const [instanceForm] = Form.useForm<InstanceForm>()

  useEffect(() => {
    const dismissed = localStorage.getItem(STORAGE_KEY)
    if (dismissed) return

    const load = async () => {
      try {
        const res: any = await hostApi.list(1000, 0)
        const items = res.data || []
        setHosts(items)
        if (items.length > 0) {
          setSelectedHost(items[0])
        }
      } catch {
        // ignore
      }
    }
    load().then(() => setVisible(true))
  }, [])

  const handleAddHost = async () => {
    try {
      const values = await hostForm.validateFields()
      const res: any = await hostApi.create({
        name: values.name || values.address,
        address: values.address,
        ssh_port: values.ssh_port || 22,
        ssh_user: values.ssh_user || 'root',
        ssh_credential: values.ssh_password || '',
        agent_port: values.agent_port || 9090,
      })
      const host = res.data
      message.success('主机添加成功')
      setHosts((prev) => [...prev, host])
      setSelectedHost(host)
      setCurrentStep(1)
    } catch {
      // interceptor already showed error
    }
  }

  const handleInstallAgent = async () => {
    if (!selectedHost) return
    setSubmitting(true)
    try {
      await hostApi.agentAction(selectedHost.id, 'install', selectedHost.agent_port, false)
      message.success('Agent 安装任务已提交，请在主机详情页查看进度')
      setCurrentStep(2)
    } catch {
      // ignore
    } finally {
      setSubmitting(false)
    }
  }

  const handleScan = async () => {
    if (!selectedHost) return
    setScanning(true)
    setScannedInstances([])
    try {
      const res: any = await hostApi.scanInstances(selectedHost.id, { probe_mysql: true })
      const taskId = res.data?.task_id
      if (!taskId) {
        message.warning('扫描任务未返回 task_id')
        setScanning(false)
        return
      }
      message.info('扫描任务已提交，正在轮询结果...')
      const timer = window.setInterval(async () => {
        try {
          const r: any = await hostApi.getScanResult(selectedHost.id, taskId)
          const data = r?.data
          if (!data) return
          if (data.status === 'success') {
            window.clearInterval(timer)
            setScannedInstances(data.instances || [])
            setScanning(false)
            if (data.instances?.length > 0) {
              message.success(`发现 ${data.instances.length} 个实例`)
            } else {
              message.warning('未发现待纳管实例')
            }
          }
          if (data.status === 'failed') {
            window.clearInterval(timer)
            setScanning(false)
            message.error(`扫描失败: ${data.error || data.message || '未知错误'}`)
          }
        } catch {
          // keep polling
        }
      }, 2000)
    } catch {
      setScanning(false)
      message.error('扫描发起失败')
    }
  }

  const handleRegisterSelected = async () => {
    if (!selectedHost || selectedInstances.length === 0) return
    setSubmitting(true)
    try {
      const instances = selectedInstances.map((item) => ({
        port: item.port,
        name: item.recommended_name || `${selectedHost.name}-${item.port}`,
        username: 'root',
        password: '',
        host: selectedHost.address,
        host_id: selectedHost.id,
        flavor: item.flavor,
        version: item.version,
        full_version: item.version_full,
      }))
      await hostApi.registerScannedInstances(selectedHost.id, instances)
      message.success(`已纳管 ${instances.length} 个实例`)
      setSelectedInstances([])
      setScannedInstances([])
      setCurrentStep(3)
    } catch {
      // ignore
    } finally {
      setSubmitting(false)
    }
  }

  const handleDeployInstance = async () => {
    try {
      const values = await instanceForm.validateFields()
      setSubmitting(true)
      await instanceApi.create({
        name: values.name,
        host: values.host,
        port: values.port,
        username: values.username,
        password: values.password,
        host_id: selectedHost?.id,
        basedir: values.basedir,
        datadir: values.datadir,
        os_user: values.os_user || 'mysql',
      })
      message.success('实例创建成功，请在实例管理页执行部署')
      localStorage.setItem(STORAGE_KEY, 'true')
      setVisible(false)
    } catch {
      // ignore
    } finally {
      setSubmitting(false)
    }
  }

  const handleDismiss = () => {
    localStorage.setItem(STORAGE_KEY, 'true')
    setVisible(false)
  }

  const renderStepContent = () => {
    switch (currentStep) {
      case 0:
        return (
          <Form form={hostForm} layout="vertical" initialValues={{ ssh_port: 22, agent_port: 9090, ssh_user: 'root' }}>
            <Form.Item name="name" label="主机名称" rules={[{ required: true, message: '请输入主机名称' }]}>
              <Input placeholder="例如: db-host-01" />
            </Form.Item>
            <Form.Item name="address" label="主机地址" rules={[{ required: true, message: '请输入主机地址' }]}>
              <Input placeholder="例如: 192.168.1.100" />
            </Form.Item>
            <Form.Item name="ssh_port" label="SSH 端口">
              <InputNumber min={1} max={65535} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="ssh_user" label="SSH 用户">
              <Input placeholder="root" />
            </Form.Item>
            <Form.Item name="ssh_password" label="SSH 密码" rules={[{ required: true, message: '请输入 SSH 密码' }]}>
              <Input.Password placeholder="SSH 密码" autoComplete="new-password" />
            </Form.Item>
            <Form.Item name="agent_port" label="Agent 端口">
              <InputNumber min={1} max={65535} style={{ width: '100%' }} />
            </Form.Item>
            <Button type="primary" icon={<PlusOutlined />} block onClick={handleAddHost}>
              添加主机
            </Button>
          </Form>
        )
      case 1:
        return (
          <div>
            <p>当前主机：<Tag color="blue">{selectedHost?.name} ({selectedHost?.address})</Tag></p>
            <Button type="primary" icon={<ThunderboltOutlined />} block onClick={handleInstallAgent} loading={submitting}>
              安装 Agent
            </Button>
          </div>
        )
      case 2:
        return (
          <div>
            <p>当前主机：<Tag color="blue">{selectedHost?.name} ({selectedHost?.address})</Tag></p>
            <Button type="primary" icon={<ScanOutlined />} block onClick={handleScan} loading={scanning}>
              扫描实例
            </Button>
            {scannedInstances.length > 0 && (
              <Table
                rowSelection={{
                  onChange: (keys: React.Key[], rows: ScannedInstance[]) => setSelectedInstances(rows),
                }}
                dataSource={scannedInstances}
                rowKey={(row) => `${row.port}-${row.flavor}`}
                pagination={false}
                size="small"
                style={{ marginTop: 12 }}
                columns={[
                  { title: '端口', dataIndex: 'port', key: 'port' },
                  { title: '引擎', dataIndex: 'flavor', key: 'flavor' },
                  { title: '版本', dataIndex: 'version', key: 'version' },
                  { title: '运行中', dataIndex: 'running', key: 'running', render: (v) => (v ? <Tag color="success">是</Tag> : <Tag>否</Tag>) },
                ]}
              />
            )}
            {selectedInstances.length > 0 && (
              <Button type="primary" icon={<RocketOutlined />} block onClick={handleRegisterSelected} loading={submitting} style={{ marginTop: 12 }}>
                纳管选中实例 ({selectedInstances.length})
              </Button>
            )}
          </div>
        )
      case 3:
        return (
          <Form form={instanceForm} layout="vertical" initialValues={{ port: 3306, username: 'root', os_user: 'mysql' }}>
            <Form.Item name="name" label="实例名称" rules={[{ required: true, message: '请输入实例名称' }]}>
              <Input placeholder="例如: order-db-01" />
            </Form.Item>
            <Form.Item name="host" label="连接地址" rules={[{ required: true, message: '请输入连接地址' }]}>
              <Input placeholder={selectedHost?.address} />
            </Form.Item>
            <Form.Item name="port" label="端口" rules={[{ required: true, message: '请输入端口' }]}>
              <InputNumber min={1} max={65535} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
              <Input placeholder="root" />
            </Form.Item>
            <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
              <Input.Password placeholder="MySQL 密码" autoComplete="new-password" />
            </Form.Item>
            <Form.Item name="basedir" label="basedir">
              <Input placeholder="/opt/mysql-8.0.36" />
            </Form.Item>
            <Form.Item name="datadir" label="datadir">
              <Input placeholder="/data/mysql/3307" />
            </Form.Item>
            <Form.Item name="os_user" label="OS 用户">
              <Input placeholder="mysql" />
            </Form.Item>
            <Button type="primary" icon={<RocketOutlined />} block onClick={handleDeployInstance} loading={submitting}>
              创建实例
            </Button>
          </Form>
        )
      default:
        return null
    }
  }

  if (!visible) return null

  return (
    <Modal
      title="快速部署向导"
      open={visible}
      onCancel={handleDismiss}
      footer={<Button onClick={handleDismiss}>稍后再说</Button>}
      width={Math.min(window.innerWidth * 0.96, 640)}
      styles={{ body: { maxHeight: 'calc(100vh - 180px)', overflowY: 'auto' } }}
    >
      <Steps
        current={currentStep}
        size="small"
        style={{ marginBottom: 16 }}
        items={[
          { title: '添加主机' },
          { title: '安装 Agent' },
          { title: '扫描实例' },
          { title: '创建实例' },
        ]}
      />
      <div style={{ minHeight: 240 }}>{renderStepContent()}</div>
    </Modal>
  )
}

export default FirstDeployWizard
