import React, { useEffect, useState } from 'react'
import { Card, Button, Space, Table, message, Tag, Progress, Divider, Modal, Tabs } from 'antd'
import { CheckCircleOutlined, SwapOutlined, StopOutlined } from '@ant-design/icons'
import { migrationApi, instanceApi } from '../services/api'
import MigrationFormSection from '../components/MigrationFormSection'
import type { MigrationType } from '../components/MigrationFormSection'
import MigrationProgressMonitor from '../components/MigrationProgressMonitor'
import {
  MigrationTask, MigrationProgressStep, MigrationProgressResponse,
  isActiveMigrationStatus, isFailedMigrationStatus, isCompletedMigrationStatus,
  migrationStatusColor, buildProgressDetails, buildCreatePayload, taskFromResult,
} from '../services/migrationHelpers'

const MigrationManage: React.FC = () => {
  const [migrationTasks, setMigrationTasks] = useState<MigrationTask[]>([])
  const [instances, setInstances] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [currentTab, setCurrentTab] = useState('physical')
  const [activeMigration, setActiveMigration] = useState<MigrationTask | null>(null)
  const [progressDetails, setProgressDetails] = useState<MigrationProgressStep[]>([])

  const loadData = async () => {
    try {
      const [instanceRes, migrationRes]: any[] = await Promise.all([
        instanceApi.list(100, 0),
        migrationApi.list(),
      ])
      setInstances(instanceRes?.data || [])
      setMigrationTasks(migrationRes?.data || [])
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '加载迁移数据失败')
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  useEffect(() => {
    if (!activeMigration) return
    if (!isActiveMigrationStatus(activeMigration.status)) return
    const interval = setInterval(() => {
      Promise.allSettled([
        migrationApi.get(activeMigration.id),
        migrationApi.getProgress(activeMigration.id),
      ]).then(([taskResult, progressResult]) => {
        const task = taskResult.status === 'fulfilled' ? taskResult.value?.data : null
        const progress: MigrationProgressResponse | null = progressResult.status === 'fulfilled' ? progressResult.value?.data : null
        const merged = {
          ...(task || {}),
          ...(progress ? { status: progress.status, progress: progress.progress } : {}),
          id: task?.id || progress?.task_id || activeMigration.id,
          steps: Array.isArray(task?.steps) ? task.steps : (Array.isArray(progress?.steps) ? progress.steps : undefined),
          logs: Array.isArray(task?.logs) ? task.logs : (Array.isArray(progress?.logs) ? progress.logs : undefined),
        }
        if (progress) {
          setProgressDetails(buildProgressDetails(progress))
        }
        setActiveMigration((prev) => (prev ? { ...prev, ...merged } : prev))
        setMigrationTasks((tasks) => tasks.map((t) => (t.id === merged.id ? { ...t, ...merged } : t)))
        if (!isActiveMigrationStatus(merged.status)) {
          clearInterval(interval)
        }
      }).catch(() => clearInterval(interval))
    }, 2000)
    return () => clearInterval(interval)
  }, [activeMigration?.id, activeMigration?.status])

  useEffect(() => {
    if (!migrationTasks.some((task) => isActiveMigrationStatus(task.status))) return
    const interval = setInterval(loadData, 5000)
    return () => clearInterval(interval)
  }, [migrationTasks])

  const startMigration = async (values: any, strategy: MigrationType, label: string) => {
    setLoading(true)
    try {
      const apiMethod = strategy === 'physical' ? migrationApi.createPhysical
        : strategy === 'replication' ? migrationApi.createReplication
        : migrationApi.createGTID
      const res: any = await apiMethod(buildCreatePayload(values, strategy))
      const task = taskFromResult(values, strategy, res)
      if (!task) throw new Error('migration API did not return task_id')
      await loadData()
      setActiveMigration(task)
      const progress = strategy === 'physical'
        ? [
            { stage: '数据导出', progress: 0, details: '准备中...' },
            { stage: '数据传输', progress: 0, details: '等待中...' },
            { stage: '数据导入', progress: 0, details: '等待中...' },
          ]
        : strategy === 'replication'
          ? [
              { stage: '建立复制', progress: 0, details: '准备中...' },
              { stage: '数据同步', progress: 0, details: '等待中...' },
              { stage: '一致性校验', progress: 0, details: '等待中...' },
            ]
          : [
              { stage: 'GTID解析', progress: 0, details: '准备中...' },
              { stage: '事务应用', progress: 0, details: '等待中...' },
              { stage: '数据校验', progress: 0, details: '等待中...' },
            ]
      setProgressDetails(progress)
      message.success(`${label}任务已启动`)
    } catch (err: any) {
      message.error(`启动${label}失败: ` + (err?.response?.data?.message || err?.message || '未知错误'))
    } finally {
      setLoading(false)
    }
  }

  const handleVerify = async (taskId: string) => {
    message.info(`开始验证迁移任务: ${taskId}`)
    try {
      const res: any = await migrationApi.verify(taskId)
      const errors = res?.data?.errors || []
      message[errors.length > 0 ? 'warning' : 'success'](errors.length > 0 ? '迁移验证完成，但存在错误' : '迁移验证通过')
      await loadData()
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '迁移验证失败')
    }
  }

  const handleSwitch = async (taskId: string) => {
    Modal.confirm({
      title: '确认切换',
      content: '切换操作会把业务流量切到目标实例，可能导致短暂不可用，请确认已通知业务方。',
      okText: '确认切换',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          const res: any = await migrationApi.switchover(taskId)
          const data = res?.data || res
          await loadData()
          if (isFailedMigrationStatus(data?.status)) {
            message.error(data?.message || '迁移切换失败')
          } else if (isCompletedMigrationStatus(data?.status)) {
            message.success(data?.message || '迁移切换完成')
          } else {
            message.info(data?.message || '迁移切换已提交，请刷新任务状态')
          }
        } catch (err: any) {
          if (err?.response?.status === 404) {
            message.error('迁移切换接口不存在或任务不存在')
            return
          }
          message.error(err?.response?.data?.message || '切换失败')
        }
      },
    })
  }
  const handleCancel = (taskId: string) => {
    Modal.confirm({
      title: '确认取消迁移',
      content: '取消后, 已传输的数据不会自动回滚, 需手动清理。继续吗?',
      okText: '确认取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await migrationApi.cancel(taskId)
          await loadData()
          message.success('已取消迁移')
        } catch (err: any) {
          if (err?.response?.status === 404) {
            message.error('迁移取消接口不存在或任务不存在')
          } else {
            message.error(err?.response?.data?.message || '取消失败')
          }
        }
      },
    })
  }

  const columns = [
    {
      title: '任务ID',
      dataIndex: 'id',
      key: 'id',
    },
    {
      title: '错误信息',
      key: 'error',
      width: 260,
      ellipsis: true,
      render: (_: any, record: MigrationTask) => record.error || record.error_message || '-',
    },
    {
      title: '迁移类型',
      key: 'migration_type',
      render: (_: any, record: MigrationTask) => {
        const type = record.migration_type || record.strategy || ''
        const typeMap: Record<string, { color: string; text: string }> = {
          physical: { color: 'blue', text: '物理迁移' },
          replication: { color: 'green', text: '复制迁移' },
          gtid: { color: 'orange', text: 'GTID迁移' },
        }
        return <Tag color={typeMap[type]?.color}>{typeMap[type]?.text}</Tag>
      },
    },
    {
      title: '源实例',
      key: 'source_instance',
      render: (_: any, record: MigrationTask) => record.source_instance || record.source_instance_id,
    },
    {
      title: '目标实例',
      key: 'target_instance',
      render: (_: any, record: MigrationTask) => record.target_instance || record.target_instance_id,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        return <Tag color={migrationStatusColor(status)}>{status}</Tag>
      },
    },
    {
      title: '进度',
      dataIndex: 'progress',
      key: 'progress',
      render: (progress: number) => <Progress percent={progress} size="small" />,
    },
    {
      title: '开始时间',
      dataIndex: 'started_at',
      key: 'started_at',
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: MigrationTask) => (
        <Space>
          <Button
            size="small"
            icon={<CheckCircleOutlined />}
            onClick={() => handleVerify(record.id)}
            disabled={!['running', 'migrating'].includes((record.status || '').toLowerCase())}
          >
            验证
          </Button>
          <Button
            size="small"
            type="primary"
            icon={<SwapOutlined />}
            onClick={() => handleSwitch(record.id)}
            disabled={(record.status || '').toLowerCase() !== 'verifying'}
          >
            切换
          </Button>
          {isActiveMigrationStatus(record.status) && (
            <Button
              size="small"
              danger
              icon={<StopOutlined />}
              onClick={() => handleCancel(record.id)}
            >
              取消
            </Button>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <Card title="数据迁移管理">
        <Tabs
          activeKey={currentTab}
          onChange={setCurrentTab}
          items={[
            { key: 'physical', label: '物理迁移', children: <MigrationFormSection type="physical" instances={instances} loading={loading} onSubmit={(v) => startMigration(v, 'physical', '物理迁移')} /> },
            { key: 'replication', label: '复制迁移', children: <MigrationFormSection type="replication" instances={instances} loading={loading} onSubmit={(v) => startMigration(v, 'replication', '复制迁移')} /> },
            { key: 'gtid', label: 'GTID迁移', children: <MigrationFormSection type="gtid" instances={instances} loading={loading} onSubmit={(v) => startMigration(v, 'gtid', 'GTID迁移')} /> },
          ]}
        />

        {activeMigration && (
          <MigrationProgressMonitor task={activeMigration} progressDetails={progressDetails} />
        )}

        <Divider />

        <Table columns={columns} dataSource={migrationTasks} rowKey="id" loading={loading} style={{ marginTop: 16 }} />
      </Card>
    </div>
  )
}

export default MigrationManage
