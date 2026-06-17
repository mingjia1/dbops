import React, { useEffect, useRef, useState } from 'react'
import {
  Button,
  Card,
  Checkbox,
  Descriptions,
  Form,
  Input,
  InputNumber,
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
  KeyOutlined,
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

const isFailedTaskStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(normalized)
}

const isSuccessTaskStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['completed', 'success', 'succeeded', 'ok'].includes(normalized)
}

const isActiveTaskStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['pending', 'running', 'submitted', 'accepted', 'queued'].includes(normalized)
}

const formatTaskMessage = (result: any, fallback: string) => {
  const parts = [
    result?.message || fallback,
    result?.status ? `status=${result.status}` : '',
    result?.task_id ? `task_id=${result.task_id}` : '',
  ].filter(Boolean)
  return parts.join(' | ')
}

const formatBatchRows = (rows: any[]) =>
  rows
    .map((row: any) => `${row?.name || row?.instance_id || '-'}:${row?.port || '-'} ${row?.status || 'unknown'}${row?.message ? ` - ${row.message}` : ''}`)
    .join('\n')

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
  const [serviceOutput, setServiceOutput] = useState('')
  const pollRef = useRef<number | null>(null)
  const [form] = Form.useForm()
  const [userForm] = Form.useForm()
  const [grantForm] = Form.useForm()
  const [passwordForm] = Form.useForm()
  const [batchPasswordForm] = Form.useForm()
  const [variableForm] = Form.useForm()
  const [configForm] = Form.useForm()
  const [serviceForm] = Form.useForm()
  const [forceResetOpen, setForceResetOpen] = useState(false)
  const [forceResetting, setForceResetting] = useState(false)
  const [forceResetForm] = Form.useForm()

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
      if (isFailedTaskStatus(result?.status)) {
        Modal.error({
          title: '\u64cd\u4f5c\u5931\u8d25',
          content: formatTaskMessage(result, '\u64cd\u4f5c\u5931\u8d25'),
        })
      } else if (isActiveTaskStatus(result?.status)) {
        Modal.info({
          title: '\u4efb\u52a1\u5df2\u63d0\u4ea4',
          content: formatTaskMessage(result, '\u4efb\u52a1\u5df2\u63d0\u4ea4\uff0c\u8bf7\u7a0d\u540e\u5237\u65b0\u67e5\u770b\u7ed3\u679c'),
        })
      } else if (!result?.status || isSuccessTaskStatus(result?.status)) {
        message.success('\u64cd\u4f5c\u5b8c\u6210')
      } else {
        Modal.warning({
          title: '\u64cd\u4f5c\u72b6\u6001\u672a\u786e\u8ba4',
          content: formatTaskMessage(result, '\u540e\u7aef\u8fd4\u56de\u4e86\u672a\u8bc6\u522b\u7684\u72b6\u6001\uff0c\u8bf7\u5237\u65b0\u540e\u786e\u8ba4\u7ed3\u679c'),
        })
      }
      return result
    } catch (err: any) {
      Modal.error({
        title: '\u64cd\u4f5c\u5931\u8d25',
        content: err?.response?.data?.message || err?.message || '\u64cd\u4f5c\u5931\u8d25',
      })
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
    const content = result?.data?.content ?? result?.data?.output ?? result?.message
    const path = result?.data?.path
    if (content) {
      configForm.setFieldsValue({ content, ...(path ? { path } : {}) })
    }
  }

  const runServiceControl = async (values: any) => {
    setServiceOutput('')
    const result = await runAdmin({ action: 'service_control', ...values })
    const output = result?.data?.output ?? result?.message ?? ''
    if (output) {
      setServiceOutput(output)
      Modal.info({
        title: '服务操作结果',
        content: <div style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{output}</div>,
      })
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
          const rows = result?.data?.rows || []
          if (isSuccessTaskStatus(result?.status)) {
            message.success('批量密码修改完成')
          } else if (isFailedTaskStatus(result?.status)) {
            Modal.error({
              title: '批量密码修改失败',
              content: (
                <div style={{ maxHeight: 260, overflow: 'auto', whiteSpace: 'pre-wrap' }}>
                  <div>{formatTaskMessage(result, '批量密码修改失败')}</div>
                  {rows.length > 0 && <div style={{ marginTop: 12 }}>{formatBatchRows(rows)}</div>}
                </div>
              ),
            })
          } else {
            Modal.warning({
              title: '批量密码修改状态未确认',
              content: (
                <div style={{ maxHeight: 260, overflow: 'auto', whiteSpace: 'pre-wrap' }}>
                  <div>{formatTaskMessage(result, '批量密码修改状态未确认')}</div>
                  {rows.length > 0 && <div style={{ marginTop: 12 }}>{formatBatchRows(rows)}</div>}
                </div>
              ),
            })
          }
        } catch (err: any) {
          Modal.error({
            title: '批量密码修改失败',
            content: err?.response?.data?.message || err?.message || '批量密码修改失败',
          })
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
    form.setFieldsValue({
      name: instance.name,
      cluster_id: instance.cluster_id,
      host: instance.connection?.host,
      port: instance.connection?.port,
      username: instance.connection?.username,
      password: '',
      ssl_enabled: instance.connection?.ssl_enabled,
      basedir: instance.connection?.basedir,
      datadir: instance.connection?.datadir,
      os_user: instance.connection?.os_user,
      package_url: instance.connection?.package_url,
      version_id: instance.connection?.version_id,
    })
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

  const checkPasswordComplexity = (pw: string) => {
    const errors: string[] = []
    if (pw.length < 8) errors.push('至少8位')
    if (!/[A-Z]/.test(pw)) errors.push('大写字母')
    if (!/[a-z]/.test(pw)) errors.push('小写字母')
    if (!/[0-9]/.test(pw)) errors.push('数字')
    if (!/[^A-Za-z0-9]/.test(pw)) errors.push('特殊字符')
    return errors
  }

  const getPasswordStrength = (pw: string) => {
    if (!pw) return { level: 0, label: '', color: '' }
    let score = 0
    if (/[A-Z]/.test(pw)) score++
    if (/[a-z]/.test(pw)) score++
    if (/[0-9]/.test(pw)) score++
    if (/[^A-Za-z0-9]/.test(pw)) score++
    if (pw.length >= 8) score++
    if (score <= 2) return { level: score, label: '弱', color: 'red' }
    if (score <= 3) return { level: score, label: '中', color: 'orange' }
    if (score <= 4) return { level: score, label: '强', color: 'lime' }
    return { level: score, label: '非常强', color: 'green' }
  }

  const handleForceReset = async () => {
    if (!id) return
    const values = await forceResetForm.validateFields()
    const useDefaultPassword = values.use_default_password === true
    if (!useDefaultPassword) {
      if (values.new_password !== values.confirm_password) {
        message.error('两次输入的密码不一致')
        return
      }
      const errors = checkPasswordComplexity(values.new_password)
      if (errors.length > 0) {
        message.error(`密码复杂度不足: ${errors.join(', ')}`)
        return
      }
    }
    setForceResetting(true)
    try {
      await instanceApi.forceResetPassword(id, {
        username: values.username || 'root',
        user_host: values.user_host || '%',
        new_password: useDefaultPassword ? undefined : values.new_password,
      })
      message.success('密码强制重置成功，平台连接密码已同步更新')
      setForceResetOpen(false)
      forceResetForm.resetFields()
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '强制重置密码失败')
    } finally {
      setForceResetting(false)
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
          <Button icon={<KeyOutlined />} onClick={() => setForceResetOpen(true)}>
            强制重置密码
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
                        <Space direction="vertical" style={{ width: '100%' }} size={12}>
                          <Form form={serviceForm} layout="inline" initialValues={{ verb: 'status' }} onFinish={runServiceControl}>
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
                          {serviceOutput && (
                            <Input.TextArea value={serviceOutput} rows={4} readOnly style={{ fontFamily: 'monospace' }} />
                          )}
                        </Space>
                      ),
                    },
                  ]}
                />
              </Space>
            ),
          },
        ]}
      />

      <Modal title="编辑实例" open={editOpen} onCancel={() => setEditOpen(false)} onOk={submitEdit} confirmLoading={submitting} okText="保存" cancelText="取消" width={640}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="实例名称" rules={[{ required: true, message: '请输入实例名称' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="cluster_id" label="集群 ID">
            <Input placeholder="例如 mgr-cluster-01" />
          </Form.Item>
          <Form.Item name="host" label="连接地址" rules={[{ required: true, message: '请输入连接地址' }]}>
            <Input placeholder="例如 10.1.81.16" />
          </Form.Item>
          <Form.Item name="port" label="端口" rules={[{ required: true, message: '请输入端口' }]}>
            <InputNumber min={1} max={65535} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="username" label="连接账号" rules={[{ required: true, message: '请输入连接账号' }]}>
            <Input placeholder="root" />
          </Form.Item>
          <Form.Item name="password" label="连接密码" extra="留空表示不修改平台保存的实例连接密码">
            <Input.Password placeholder="输入后仅更新平台连接信息" autoComplete="new-password" />
          </Form.Item>
          <Form.Item name="ssl_enabled" valuePropName="checked">
            <Checkbox>启用 SSL</Checkbox>
          </Form.Item>
          <Form.Item name="basedir" label="basedir">
            <Input placeholder="/opt/mysql 或 /opt/dbops-pxc/usr" />
          </Form.Item>
          <Form.Item name="datadir" label="datadir">
            <Input placeholder="/data/mysql/3306" />
          </Form.Item>
          <Form.Item name="os_user" label="OS 用户">
            <Input placeholder="mysql" />
          </Form.Item>
          <Form.Item name="version_id" label="版本 ID">
            <Input placeholder="mysql-8.0.36" />
          </Form.Item>
          <Form.Item name="package_url" label="package_url">
            <Input />
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

      <Modal
        title="强制重置 MySQL root 密码"
        open={forceResetOpen}
        onCancel={() => { setForceResetOpen(false); forceResetForm.resetFields() }}
        onOk={handleForceReset}
        confirmLoading={forceResetting}
        okText="确认重置"
        cancelText="取消"
        okButtonProps={{ danger: true }}
        width={500}
      >
        <div style={{ marginBottom: 16, color: '#faad14' }}>
          注意：此操作将通过 SSH 跳过权限验证强制重置 MySQL root 密码。重置后平台连接密码将同步更新。
        </div>
        <Form form={forceResetForm} layout="vertical" autoComplete="off" initialValues={{ username: 'root', user_host: '%', use_default_password: true }}>
          <Form.Item name="use_default_password" valuePropName="checked">
            <Checkbox>使用部署默认 MySQL 密码</Checkbox>
          </Form.Item>
          <Form.Item name="username" label="账号" rules={[{ required: true, message: '请输入账号' }]}>
            <Input placeholder="root" />
          </Form.Item>
          <Form.Item name="user_host" label="账号 Host" rules={[{ required: true, message: '请输入账号 Host' }]}>
            <Input placeholder="%" />
          </Form.Item>
          <Form.Item
            name="new_password"
            label="新密码"
            dependencies={['use_default_password']}
            rules={[
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (getFieldValue('use_default_password') || value) return Promise.resolve()
                  return Promise.reject(new Error('请输入新密码'))
                },
              }),
              { min: 8, message: '密码至少8位' },
              { pattern: /[A-Z]/, message: '需要大写字母' },
              { pattern: /[a-z]/, message: '需要小写字母' },
              { pattern: /[0-9]/, message: '需要数字' },
              { pattern: /[^A-Za-z0-9]/, message: '需要特殊字符' },
            ]}
          >
            <Input.Password
              placeholder="VFR$3edcXSW@1qaz"
              autoComplete="new-password"
              onChange={(e) => {
                const pw = e.target.value
                const strength = getPasswordStrength(pw)
                forceResetForm.setFieldsValue({ _strength: strength })
                forceResetForm.setFieldsValue({ _errors: checkPasswordComplexity(pw) })
              }}
            />
          </Form.Item>
          <Form.Item
            name="confirm_password"
            label="确认密码"
            dependencies={['new_password', 'use_default_password']}
            rules={[
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (getFieldValue('use_default_password')) return Promise.resolve()
                  if (!value) return Promise.reject(new Error('请再次输入新密码'))
                  if (!value || getFieldValue('new_password') === value) return Promise.resolve()
                  return Promise.reject(new Error('两次输入的密码不一致'))
                },
              }),
            ]}
          >
            <Input.Password placeholder="再次输入新密码" autoComplete="new-password" />
          </Form.Item>
          <Form.Item shouldUpdate={(prev, cur) => prev.new_password !== cur.new_password}>
            {({ getFieldValue }) => {
              const pw = getFieldValue('new_password') || ''
              const errors = checkPasswordComplexity(pw)
              const strength = getPasswordStrength(pw)
              return (
                <div>
                  <div style={{ marginBottom: 4 }}>
                    密码强度：
                    <span style={{ color: strength.color, fontWeight: 'bold' }}>
                      {strength.label || '未设置'}
                    </span>
                  </div>
                  {errors.length > 0 && (
                    <div style={{ fontSize: 12, color: '#ff4d4f' }}>
                      缺少：{errors.join(', ')}
                    </div>
                  )}
                </div>
              )
            }}
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}

export default InstanceDetail
