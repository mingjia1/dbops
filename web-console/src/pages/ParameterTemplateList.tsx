import React, { useEffect, useState } from 'react'
import { Card, Table, Button, Space, Tag, message, Modal } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, EyeOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { parameterTemplateApi, type ParameterTemplate } from '../services/api'

const ParameterTemplateList: React.FC = () => {
  const [data, setData] = useState<any[]>([])
  const [loading, setLoading] = useState(false)

  const fetchData = () => {
    setLoading(true)
    parameterTemplateApi.list().then((res: any) => {
      setData(res?.data || [])
    }).catch(() => {}).finally(() => setLoading(false))
  }

  useEffect(() => { fetchData() }, [])

  const handleDelete = (id: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除此参数模板吗？',
      onOk: () => parameterTemplateApi.delete(id).then(() => {
        message.success('删除成功')
        fetchData()
      }).catch(() => {}),
    })
  }

  const columns: ColumnsType<ParameterTemplate> = [
    { title: '模板名称', dataIndex: 'name', key: 'name' },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      render: (v) => <Tag color="blue">{v || 'v1.0'}</Tag>,
    },
    { title: '描述', dataIndex: 'description', key: 'description' },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (t) => t || '-',
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Space>
          <Button type="link" size="small" icon={<EyeOutlined />}>查看</Button>
          <Button type="link" size="small" icon={<EditOutlined />}>编辑</Button>
          <Button type="link" size="small" icon={<EditOutlined />}>应用</Button>
          <Button type="link" size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record.id)}>删除</Button>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title="参数模板"
        extra={
          <Button type="primary" icon={<PlusOutlined />}>
            创建模板
          </Button>
        }
      >
        <Table columns={columns} dataSource={data} rowKey="id" loading={loading} />
      </Card>
    </div>
  )
}

export default ParameterTemplateList