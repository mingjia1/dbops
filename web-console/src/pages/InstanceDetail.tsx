import React, { useEffect, useRef, useState } from 'react'
import {
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  Modal,
  Popconfirm,
  Progress,
  Select,
  Space,
  Spin,
  Table,
  Tabs,
  Tag,
  message,
} from 'antd'
import {
  ArrowLeftOutlined,
  DeleteOutlined,
  EditOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { useNavigate, useParams } from 'react-router-dom'
import { instanceApi, type Instance as InstanceModel } from '../services/api'

interface AdminRow {
  [key: string]: string
}

const InstanceDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [instance, setInstance] = useState<InstanceModel | null>(null)
  const [loading, setLoading] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [deploying, setDeploying] = useState(false)
  const [deployOpen, setDeployOpen] = useState(false)
  const [deployProgress, setDeployProgress] = useState(0)
  const [deployStage, setDeployStage] = useState('已提交')
  const [deployStatus, setDeployStatus] = useState<'running' | 'success' | 'failed' | null>(null)
  const [deployMessage, setDeployMessage] = useState('')
  const [adminLoading, setAdminLoading] = useState(false)
  const [users, setUsers] = useState<AdminRow[]>([])
  const [variables, setVariables] = useState<AdminRow[]>([])
  const pollRef = useRef<number | null>(null)
  const [form] = Form.useForm()
  const [userForm] = Form.useForm()
  const [grantForm] = Form.useForm()
  const [passwordForm] = Form.useForm()
  const [batchPasswordForm] = Form.useForm()
  const [variableForm] = Form.useForm()
  const [configForm] = Form.useForm()
  const [serviceForm] = Form.useForm()

  useEffect(() => {
    fetchInstance()
  }, [id])

  useEffect(() => () => stopDeployPolling(), [])

  const fetchInstance = async () => {
    if (!id) return
    setLoading(true)
    try {
      const response: any = await instanceApi.get(id)
      setInstance(response.data)
    } catch {
      message.error('获取实例信息失败')
    } finally {
      setLoading(false)
    }
  }

  const runAdmin = async (payload: any) => {
    if (!id) return null
    setAdminLoading(true)
    try {
      const response: any = await instanceApi.adminAction(id, payload)
      const result = response?.data
      if (result?.status === 'failed') {
        message.error(result?.message || '操作失败')
      } else {
        message.success('操作完成')
      }
      return result
    } catch (err: any) {
      message.error(err?.response?.data?.message || '操作失败')
      return null
    } finally {
      setAdminLoading(false)
    }
  }

  const loadUsers = async () => {
    const result = await runAdmin({ action: 'list_users' })
    setUsers(result?.data?.rows || [])
  }

  const loadVariables = async () => {
    const values = variableForm.getFieldsValue()
    const result = await runAdmin({ action: 'show_variables', pattern: values.pattern || '%' })
    setVariables(result?.data?.rows || [])
  }

  const readConfig = async () => {
    const values = configForm.getFieldsValue()
    const result = await runAdmin({ action: 'read_config', path: values.path || '/etc/my.cnf' })
    if (result?.message) {
      configForm.setFieldsValue({ content: result.message })
    }
  }

  const batchUpdatePassword = async () => {
    const values = await batchPasswordForm.validateFields()
    const ports = String(values.ports || '')
      .split(',')
      .map((item) => Number(item.trim()))
      .filter((port) => Number.isInteger(port) && port > 0)
    if (!ports.length) {
      message.error('请输入有效端口')
      return
    }
    Modal.confirm({
      title: '确认批量修改密码',
      content: `将修改 ${values.host} 上端口 ${ports.join(', ')} 的 ${values.username}@${values.user_host || '%'} 密码，并同步平台保存的连接密码。`,
      okText: '确认修改',
      onOk: async () => {
        setAdminLoading(true)
        try {
          const response: any = await instanceApi.batchUpdatePassword({
            host: values.host,
            ports,
            username: values.username,
            user_host: values.user_host || '%',
            current_password: values.current_password || '',
            new_password: values.new_password,
            update_stored: true,
          })
          const result = response?.data
          if (result?.status === 'completed') {
            message.success('批量密码修改完成')
          } else {
            message.warning(result?.message || '部分实例修改失败')
          }
        } catch (err: any) {
          message.error(err?.response?.data?.message || '批量密码修改失败')
        } finally {
          setAdminLoading(false)
        }
      },
    })
  }

  const writeConfig = async () => {
    const values = await configForm.validateFields()
    Modal.confirm({
      title: '确认写入配置文件',
      content: '写入前会在目标主机保留 .bak 时间戳备份。配置变更通常需要重启服务才生效。',
      okText: '写入',
      okButtonProps: { danger: true },
      onOk: () => runAdmin({ action: 'write_config', path: values.path, content: values.content }),
    })
  }

  const openEdit = () => {
    if (!instance) return
    form.setFieldsValue({ name: instance.name, cluster_id: instance.cluster_id })
    setEditOpen(true)
  }

  const submitEdit = async () => {
    if (!id) return
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      await instanceApi.update(id, values)
      message.success('更新成功')
      setEditOpen(false)
      fetchInstance()
    } finally {
      setSubmitting(false)
    }
  }

  const stopDeployPolling = () => {
    if (pollRef.current) {
      window.clearInterval(pollRef.current)
      pollRef.current = null
    }
  }

  const startDeployPolling = () => {
    stopDeployPolling()
    let attempts = 0
    pollRef.current = window.setInterval(async () => {
      attempts += 1
      try {
        const res: any = await instanceApi.get(id!)
        const status = res?.data?.status
        if (!status) return
        if (typeof status.deploy_progress === 'number') setDeployProgress(status.deploy_progress)
        if (status.stage) setDeployStage(status.stage)
        if (status.deploy_status) setDeployStatus(status.deploy_status)
        if (status.deploy_message) setDeployMessage(status.deploy_message)
        if (status.deploy_status === 'success') {
          setDeployStatus('success')
          setDeployProgress(100)
          setDeployStage('部署完成')
          stopDeployPolling()
          fetchInstance()
        } else if (status.deploy_status === 'failed' || attempts > 600) {
          setDeployStatus(status.deploy_status === 'failed' ? 'failed' : null)
          stopDeployPolling()
        }
      } catch {
        // keep polling transient failures
      }
    }, 2000)
  }

  const handleDeploy = async () => {
    if (!id) return
    setDeploying(true)
    setDeployOpen(true)
    setDeployProgress(0)
    setDeployStage('已提交，等待后端执行')
    setDeployStatus('running')
    setDeployMessage('')
    try {
      await instanceApi.deploy(id)
      message.success('部署任务已提交')
      startDeployPolling()
    } catch {
      setDeployStatus('failed')
      setDeployMessage('提交失败')
    } finally {
      setDeploying(false)
    }
  }

  const handleDelete = async () => {
    if (!id) return
    await instanceApi.delete(id)
    message.success('删除成功')
    navigate('/dashboard/instances')
  }

  const version = instance?.version
  const hasVersion = !!version?.full_version

  if (loading) return <Spin style={{ display: 'block', margin: '100px auto' }} />
  if (!instance) return <Card>实例不存在</Card>

  const userColumns: ColumnsType<AdminRow> = [
    { title: '用户', dataIndex: 'user', key: 'user' },
    { title: 'Host', dataIndex: 'host', key: 'host' },
    { title: '认证插件', dataIndex: 'plugin', key: 'plugin' },
    { title: '锁定', dataIndex: 'account_locked', key: 'account_locked' },
  ]

  const variableColumns: ColumnsType<AdminRow> = [
    { title: '参数', dataIndex: 'name', key: 'name' },
    { title: '当前值', dataIndex: 'value', key: 'value' },
  ]

  return (
    <Card
      title={
        <Space>
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/dashboard/instances')}>
            返回列表
          </Button>
          <span>实例详情：{instance.name}</span>
        </Space>
      }
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchInstance}>
            刷新
          </Button>
          <Button icon={<ThunderboltOutlined />} loading={deploying} onClick={handleDeploy}>
            部署
          </Button>
          <Button icon={<EditOutlined />} onClick={openEdit}>编辑</Button>
          <Popconfirm title="确定删除此实例？" onConfirm={handleDelete} okText="确定" cancelText="取消">
            <Button danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      }
    >
      <Tabs
        defaultActiveKey="basic"
        items={[
          {
            key: 'basic',
            label: '基本信息',
            children: (
              <Descriptions bordered column={2}>
                <Descriptions.Item label="实例 ID">{instance.id}</Descriptions.Item>
                <Descriptions.Item label="实例名称">{instance.name}</Descriptions.Item>
                <Descriptions.Item label="主机地址">{instance.connection?.host || '-'}</Descriptions.Item>
                <Descriptions.Item label="端口">{instance.connection?.port || '-'}</Descriptions.Item>
                <Descriptions.Item label="连接用户">{instance.connection?.username || '-'}</Descriptions.Item>
                <Descriptions.Item label="SSL">
                  <Tag color={instance.connection?.ssl_enabled ? 'success' : 'default'}>{instance.connection?.ssl_enabled ? '启用' : '未启用'}</Tag>
                </Descriptions.Item>
                <Descriptions.Item label="集群 ID" span={2}>{instance.cluster_id || <Tag>单点</Tag>}</Descriptions.Item>
                <Descriptions.Item label="运行状态">{instance.status?.run_status || '-'}</Descriptions.Item>
                <Descriptions.Item label="健康状态">{instance.status?.health_status || '-'}</Descriptions.Item>
                <Descriptions.Item label="角色">{instance.status?.role || '-'}</Descriptions.Item>
                <Descriptions.Item label="复制延迟">{instance.status?.seconds_behind_master ?? '-'}</Descriptions.Item>
                <Descriptions.Item label="版本">
                  {hasVersion ? <Tag color="blue">{version.flavor} {version.version}</Tag> : '-'}
                </Descriptions.Item>
                <Descriptions.Item label="完整版本">{version?.full_version || '-'}</Descriptions.Item>
                <Descriptions.Item label="LTS">
                  {hasVersion ? <Tag color={version.is_lts ? 'success' : 'default'}>{version.is_lts ? '是' : '否'}</Tag> : '-'}
                </Descriptions.Item>
                <Descriptions.Item label="EOL 日期">{hasVersion && version.eol_date ? new Date(version.eol_date).toLocaleDateString() : '-'}</Descriptions.Item>
                <Descriptions.Item label="安装目录">{instance.connection?.basedir || '-'}</Descriptions.Item>
                <Descriptions.Item label="数据目录">{instance.connection?.datadir || '-'}</Descriptions.Item>
                <Descriptions.Item label="创建时间">{new Date(instance.created_at).toLocaleString()}</Descriptions.Item>
                <Descriptions.Item label="更新时间">{new Date(instance.updated_at).toLocaleString()}</Descriptions.Item>
              </Descriptions>
            ),
          },
          {
            key: 'admin',
            label: '全局管理',
            children: (
              <Space direction="vertical" style={{ width: '100%' }} size={16}>
                <Tabs
                  items={[
                    {
                      key: 'users',
                      label: '用户/权限/密码',
                      children: (
                        <Space direction="vertical" style={{ width: '100%' }} size={12}>
                          <Space>
                            <Button icon={<ReloadOutlined />} onClick={loadUsers} loading={adminLoading}>加载用户</Button>
                          </Space>
                          <Table columns={userColumns} dataSource={users.map((row, index) => ({ ...row, key: String(index) }))} size="small" pagination={false} />
                          <Form form={userForm} layout="inline" onFinish={(values) => runAdmin({ action: 'create_user', ...values }).then(loadUsers)}>
                            <Form.Item name="username" rules={[{ required: true }]}><Input placeholder="用户名" /></Form.Item>
                            <Form.Item name="user_host" initialValue="%"><Input placeholder="Host" /></Form.Item>
                            <Form.Item name="password" rules={[{ required: true }]}><Input.Password placeholder="密码" /></Form.Item>
                            <Button htmlType="submit" type="primary" loading={adminLoading}>创建用户</Button>
                          </Form>
                          <Form form={passwordForm} layout="inline" onFinish={(values) => runAdmin({ action: 'change_password', update_stored_password: true, ...values })}>
                            <Form.Item name="username" rules={[{ required: true }]}><Input placeholder="用户名" /></Form.Item>
                            <Form.Item name="user_host" initialValue="%"><Input placeholder="Host" /></Form.Item>
                            <Form.Item name="password" rules={[{ required: true }]}><Input.Password placeholder="新密码" /></Form.Item>
                            <Button htmlType="submit" loading={adminLoading}>修改密码</Button>
                          </Form>
                          <Card size="small" title="批量预配置密码生效">
                            <Form
                              form={batchPasswordForm}
                              layout="inline"
                              initialValues={{ host: '10.1.81.41', ports: '3307,3308', username: 'root', user_host: '%' }}
                            >
                              <Form.Item name="host" rules={[{ required: true }]}>
                                <Input placeholder="主机地址" style={{ width: 140 }} />
                              </Form.Item>
                              <Form.Item name="ports" rules={[{ required: true }]}>
                                <Input placeholder="端口,逗号分隔" style={{ width: 140 }} />
                              </Form.Item>
                              <Form.Item name="username" rules={[{ required: true }]}>
                                <Input placeholder="用户名" style={{ width: 110 }} />
                              </Form.Item>
                              <Form.Item name="user_host">
                                <Input placeholder="Host" style={{ width: 90 }} />
                              </Form.Item>
                              <Form.Item name="current_password">
                                <Input.Password placeholder="当前密码(可空)" style={{ width: 160 }} />
                              </Form.Item>
                              <Form.Item name="new_password" rules={[{ required: true }]}>
                                <Input.Password placeholder="新密码" style={{ width: 160 }} />
                              </Form.Item>
                              <Button onClick={batchUpdatePassword} loading={adminLoading}>批量修改并同步</Button>
                            </Form>
                          </Card>
                          <Form form={grantForm} layout="inline" onFinish={(values) => runAdmin({ action: 'grant_privileges', ...values })}>
                            <Form.Item name="username" rules={[{ required: true }]}><Input placeholder="用户名" /></Form.Item>
                            <Form.Item name="user_host" initialValue="%"><Input placeholder="Host" /></Form.Item>
                            <Form.Item name="privileges" initialValue="SELECT"><Input placeholder="权限，如 SELECT, INSERT" style={{ width: 180 }} /></Form.Item>
                            <Form.Item name="scope" initialValue="*.*"><Input placeholder="范围，如 db.*" /></Form.Item>
                            <Button htmlType="submit" loading={adminLoading}>授权</Button>
                            <Button onClick={() => grantForm.validateFields().then((values) => runAdmin({ action: 'revoke_privileges', ...values }))} loading={adminLoading}>回收权限</Button>
                          </Form>
                        </Space>
                      ),
                    },
                    {
                      key: 'variables',
                      label: '运行参数',
                      children: (
                        <Space direction="vertical" style={{ width: '100%' }} size={12}>
                          <Form form={variableForm} layout="inline" initialValues={{ pattern: '%' }}>
                            <Form.Item name="pattern"><Input placeholder="参数匹配，如 max_connections" style={{ width: 240 }} /></Form.Item>
                            <Button onClick={loadVariables} loading={adminLoading}>查询参数</Button>
                            <Form.Item name="name"><Input placeholder="参数名" /></Form.Item>
                            <Form.Item name="value"><Input placeholder="新值" /></Form.Item>
                            <Button onClick={() => variableForm.validateFields(['name', 'value']).then((values) => runAdmin({ action: 'set_variable', ...values }).then(loadVariables))} loading={adminLoading}>设置全局参数</Button>
                          </Form>
                          <Table columns={variableColumns} dataSource={variables.map((row, index) => ({ ...row, key: String(index) }))} size="small" pagination={{ pageSize: 10 }} />
                        </Space>
                      ),
                    },
                    {
                      key: 'config',
                      label: '配置文件',
                      children: (
                        <Form form={configForm} layout="vertical" initialValues={{ path: '/etc/my.cnf' }}>
                          <Form.Item name="path" label="配置文件路径" rules={[{ required: true }]}>
                            <Input />
                          </Form.Item>
                          <Form.Item name="content" label="配置内容" rules={[{ required: true }]}>
                            <Input.TextArea rows={14} style={{ fontFamily: 'monospace' }} />
                          </Form.Item>
                          <Space>
                            <Button onClick={readConfig} loading={adminLoading}>读取</Button>
                            <Button danger onClick={writeConfig} loading={adminLoading}>写入配置</Button>
                          </Space>
                        </Form>
                      ),
                    },
                    {
                      key: 'service',
                      label: '服务启停',
                      children: (
                        <Form form={serviceForm} layout="inline" initialValues={{ verb: 'status' }} onFinish={(values) => runAdmin({ action: 'service_control', ...values })}>
                          <Form.Item name="verb" rules={[{ required: true }]}>
                            <Select style={{ width: 140 }} options={[
                              { value: 'status', label: '状态' },
                              { value: 'start', label: '启动' },
                              { value: 'stop', label: '停止' },
                              { value: 'restart', label: '重启' },
                            ]} />
                          </Form.Item>
                          <Button htmlType="submit" icon={<PlayCircleOutlined />} loading={adminLoading}>执行</Button>
                        </Form>
                      ),
                    },
                  ]}
                />
              </Space>
            ),
          },
        ]}
      />

      <Modal title="编辑实例" open={editOpen} onCancel={() => setEditOpen(false)} onOk={submitEdit} confirmLoading={submitting} okText="保存" cancelText="取消">
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="实例名称" rules={[{ required: true, message: '请输入实例名称' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="cluster_id" label="集群 ID">
            <Input placeholder="例如 mgr-cluster-01" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`部署实例：${instance.name}`}
        open={deployOpen}
        onCancel={() => { stopDeployPolling(); setDeployOpen(false) }}
        footer={[<Button key="close" onClick={() => { stopDeployPolling(); setDeployOpen(false) }}>关闭</Button>]}
        width={600}
      >
        <div style={{ marginBottom: 8 }}>当前阶段：<b>{deployStage}</b></div>
        <Progress percent={deployProgress} status={deployStatus === 'failed' ? 'exception' : deployStatus === 'success' ? 'success' : 'active'} />
        {deployMessage && <div style={{ marginTop: 16, color: '#595959' }}>{deployMessage}</div>}
      </Modal>
    </Card>
  )
}

export default InstanceDetail
