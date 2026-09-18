import { PlusOutlined, SearchOutlined } from '@ant-design/icons'
import { Button, Col, Form, Input, InputNumber, Modal, Progress, Row, Select, Space, Statistic, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useEffect, useState } from 'react'
import { storageAPI } from '../api'
import { EntityTable } from '../components/common/EntityTable'
import { StatusBadge } from '../components/common/StatusBadge'
import { useAuth } from '../hooks/useAuth'
import { usePagination } from '../hooks/usePagination'
import { useStorageStore } from '../stores/storageStore'
import type { StorageContainer } from '../types/domain'
import { formatDateTime, temperatureLabels } from '../utils/format'

export function StoragePage() {
  const { data, loading, load } = useStorageStore()
  const pagination = usePagination()
  const { can } = useAuth()
  const [search, setSearch] = useState('')
  const [zone, setZone] = useState<string>()
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm()
  const refresh = () => load({ page: pagination.page, pageSize: pagination.pageSize, search, temperatureZone: zone })
  useEffect(() => { void refresh() }, [pagination.page, pagination.pageSize, zone])
  const create = async () => {
    const values = await form.validateFields()
    setSaving(true)
    try { await storageAPI.create(values); message.success('冻存容器创建成功'); setOpen(false); form.resetFields(); await refresh() } finally { setSaving(false) }
  }
  const columns: ColumnsType<StorageContainer> = [
    { title: '容器', fixed: 'left', render: (_, row) => <div><strong>{row.code}</strong><small className="cell-subtitle">{row.name}</small></div> },
    { title: '类型', dataIndex: 'containerType' },
    { title: '温区', dataIndex: 'temperatureZone', render: (value) => <StatusBadge value={value} /> },
    { title: '物理位置', dataIndex: 'location' },
    { title: '容量占用', width: 180, render: (_, row) => <Space direction="vertical" size={2} style={{ width: '100%' }}><span>{row.occupied} / {row.capacity}</span><Progress size="small" percent={row.capacity ? Math.round(row.occupied / row.capacity * 100) : 0} showInfo={false} status={row.occupied >= row.capacity ? 'exception' : 'normal'} /></Space> },
    { title: '运行状态', dataIndex: 'status', render: (value) => <StatusBadge value={value} dot /> },
    { title: '启用', dataIndex: 'active', render: (value) => value ? '启用' : '停用' },
    { title: '最近更新', dataIndex: 'updatedAt', render: formatDateTime },
  ]
  return (
    <div className="page-stack">
      <header className="page-header"><div><Typography.Title level={2}>冻存位置</Typography.Title><Typography.Text type="secondary">管理冷冻柜、液氮罐及其温区、容量与物理位置</Typography.Text></div>{can('storage:write') && <Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>新增容器</Button>}</header>
      <Row gutter={16}><Col xs={24} sm={8}><div className="metric"><Statistic title="容器总数" value={data.total} /></div></Col><Col xs={24} sm={8}><div className="metric"><Statistic title="可用容器" value={data.items.filter((item) => item.active && item.status === 'available').length} /></div></Col><Col xs={24} sm={8}><div className="metric"><Statistic title="需关注" value={data.items.filter((item) => item.status !== 'available').length} /></div></Col></Row>
      <div className="table-toolbar"><Input allowClear prefix={<SearchOutlined />} placeholder="搜索容器编码、名称或位置" value={search} onChange={(event) => setSearch(event.target.value)} onPressEnter={() => void refresh()} /><Select allowClear placeholder="全部温区" value={zone} onChange={setZone} options={Object.entries(temperatureLabels).map(([value, label]) => ({ value, label }))} /><Button onClick={() => void refresh()}>查询</Button></div>
      <EntityTable columns={columns} dataSource={data.items} loading={loading} emptyTitle="尚未配置冻存容器" pagination={{ current: pagination.page, pageSize: pagination.pageSize, total: data.total, onChange: pagination.update, showSizeChanger: true }} />
      <Modal title="新增冻存容器" width={640} open={open} confirmLoading={saving} onOk={() => void create()} onCancel={() => setOpen(false)} okText="创建" cancelText="取消">
        <Form form={form} layout="vertical" initialValues={{ status: 'available' }}><Row gutter={16}><Col span={12}><Form.Item name="code" label="容器编码" rules={[{ required: true, min: 2 }]}><Input placeholder="ULT-80-A01" /></Form.Item></Col><Col span={12}><Form.Item name="name" label="容器名称" rules={[{ required: true }]}><Input /></Form.Item></Col></Row><Row gutter={16}><Col span={12}><Form.Item name="containerType" label="容器类型" rules={[{ required: true }]}><Select options={[{ value: 'freezer', label: '冷冻柜' }, { value: 'cryotank', label: '液氮罐' }, { value: 'rack', label: '冻存架' }]} /></Form.Item></Col><Col span={12}><Form.Item name="temperatureZone" label="温区" rules={[{ required: true }]}><Select options={Object.entries(temperatureLabels).map(([value, label]) => ({ value, label }))} /></Form.Item></Col></Row><Form.Item name="location" label="物理位置" rules={[{ required: true }]}><Input placeholder="样本库 A 区 / 第 2 排" /></Form.Item><Row gutter={16}><Col span={12}><Form.Item name="capacity" label="总容量" rules={[{ required: true }]}><InputNumber min={1} precision={0} style={{ width: '100%' }} /></Form.Item></Col><Col span={12}><Form.Item name="status" label="运行状态" rules={[{ required: true }]}><Select options={[{ value: 'available', label: '可用' }, { value: 'maintenance', label: '维护中' }, { value: 'alarm', label: '温度告警' }]} /></Form.Item></Col></Row></Form>
      </Modal>
    </div>
  )
}
