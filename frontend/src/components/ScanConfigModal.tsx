import React from 'react'
import { Alert, Form, Input, InputNumber, Modal, Radio, Select, Space, Switch } from 'antd'
import type { Host } from '../services/api'

interface ScanConfigModalProps {
  open: boolean
  host: Host | null
  scanMode: 'default' | 'custom' | 'range'
  scanPorts: number[]
  scanRange: string
  discoverProcess: boolean
  onCancel: () => void
  onSubmit: () => void
  onScanModeChange: (mode: 'default' | 'custom' | 'range') => void
  onScanPortsChange: (ports: number[]) => void
  onScanRangeChange: (range: string) => void
  onDiscoverProcessChange: (v: boolean) => void
}

const ScanConfigModal: React.FC<ScanConfigModalProps> = ({
  open, host, scanMode, scanPorts, scanRange, discoverProcess,
  onCancel, onSubmit, onScanModeChange, onScanPortsChange, onScanRangeChange, onDiscoverProcessChange,
}) => {
  const [form] = Form.useForm()

  React.useEffect(() => {
    if (open) {
      form.setFieldsValue({ mode: scanMode, ports: scanPorts, port_range: scanRange })
    }
  }, [open, scanMode, scanPorts, scanRange, form])

  return (
    <Modal
      title={`配置扫描: ${host?.name || ''}`}
      open={open}
      onCancel={onCancel}
      onOk={onSubmit}
      okText="开始扫描"
      cancelText="取消"
      width={560}
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message="扫描说明"
        description="平台会并发 TCP 探测你指定的端口, 尝试读取 MySQL 握手包以获取版本/类型。开启进程发现后，还会通过 SSH 查询 mysqld 进程信息（需要主机已配置 SSH 凭据）。"
      />
      <Form form={form} layout="vertical">
        <Form.Item label="扫描方式" name="mode">
          <Radio.Group value={scanMode} onChange={(e) => onScanModeChange(e.target.value)}>
            <Radio.Button value="default">常用端口</Radio.Button>
            <Radio.Button value="custom">自定义端口</Radio.Button>
            <Radio.Button value="range">端口范围</Radio.Button>
          </Radio.Group>
        </Form.Item>

        {scanMode === 'default' && (
          <div style={{ marginBottom: 12, padding: 8, background: '#f0f0f0', borderRadius: 4, fontSize: 13 }}>
            将扫描 3300-3400, 33060, 33061 等常见 MySQL 端口
          </div>
        )}

        {scanMode === 'custom' && (
          <Form.Item label="自定义端口" extra="例如: 3306, 3307, 3308 (用英文逗号分隔)">
            <Input
              placeholder="3306, 3307, 3308"
              value={scanPorts.join(', ')}
              onChange={(e) => {
                const arr = e.target.value
                  .split(',')
                  .map((s) => parseInt(s.trim(), 10))
                  .filter((n) => Number.isFinite(n) && n > 0 && n <= 65535)
                onScanPortsChange(arr)
              }}
            />
          </Form.Item>
        )}

        {scanMode === 'range' && (
          <Form.Item
            label="端口范围"
            name="port_range"
            extra="支持单范围如 3306-3310, 也可混用逗号如 3306, 13306-13308"
          >
            <Input
              placeholder="3306-3310"
              value={scanRange}
              onChange={(e) => onScanRangeChange(e.target.value)}
            />
          </Form.Item>
        )}

        <Form.Item label="进程发现" extra="通过 SSH 查询主机上的 mysqld 进程, 可发现非标准端口的实例并获取 PID/内存/数据目录等信息">
          <Switch checked={discoverProcess} onChange={onDiscoverProcessChange} checkedChildren="开启" unCheckedChildren="关闭" />
        </Form.Item>
      </Form>
    </Modal>
  )
}

export default ScanConfigModal
