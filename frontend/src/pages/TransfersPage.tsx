import { CheckOutlined, CloseOutlined, HistoryOutlined, PlusOutlined, SearchOutlined, StopOutlined } from '@ant-design/icons'
import { Button, Col, Drawer, Form, Input, InputNumber, Modal, Row, Segmented, Select, Space, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useEffect, useState } from 'react'
import { specimenAPI, storageAPI, transferAPI } from '../api'
import { CustodyBadge } from '../components/common/CustodyBadge'
import { CustodyTimeline } from '../components/common/CustodyTimeline'
import { EntityTable } from '../components/common/EntityTable'
import { useAuth } from '../hooks/useAuth'
import { usePagination } from '../hooks/usePagination'
import { useTransferStore } from '../stores/transferStore'
import type { CustodyTransfer, Specimen, StorageContainer, TransferState } from '../types/domain'
import { formatDateTime } from '../utils/format'

type Resolution = 'accepted' | 'rejected' | 'cancelled'

export function TransfersPage() {
  const { data, loading, load } = useTransferStore()
  const pagination = usePagination()
  const { can } = useAuth()
  const [specimens, setSpecimens] = useState<Specimen[]>([])
  const [containers, setContainers] = useState<StorageContainer[]>([])
  const [search, setSearch] = useState('')
  const [state, setState] = useState<TransferState>()
  const [createOpen, setCreateOpen] = useState(false)
  const [resolveTarget, setResolveTarget] = useState<CustodyTransfer | null>(null)
  const [resolution, setResolution] = useState<Resolution>('accepted')
  const [timelineSpecimen, setTimelineSpecimen] = useState<Specimen | null>(null)
  const [saving, setSaving] = useState(false)
  const [createForm] = Form.useForm()
  const [resolveForm] = Form.useForm()
  const refresh = () => load({ page: pagination.page, pageSize: pagination.pageSize, search, state })
  useEffect(() => { void refresh() }, [pagination.page, pagination.pageSize, state])
  useEffect(() => {
    void Promise.all([specimenAPI.list({ page: 1, pageSize: 100 }), storageAPI.list({ page: 1, pageSize: 100 })]).then(([sampleResult, storageResult]) => {
      setSpecimens(sampleResult.items.filter((item) => !['disposed', 'released'].includes(item.state)))
      setContainers(storageResult.items.filter((item) => item.active && item.status === 'available' && item.occupied < item.capacity))
    })
  }, [])
  const create = async () => {
    const values = await createForm.validateFields()
    setSaving(true)
    try { await transferAPI.create({ ...values, reason: values.reason || '' }); message.success('交接任务已发起'); setCreateOpen(false); createForm.resetFields(); await refresh() } finally { setSaving(false) }
  }
  const resolve = async () => {
    if (!resolveTarget) return
    const values = await resolveForm.validateFields()
    setSaving(true)
    try {
      await transferAPI.resolve(resolveTarget.id, { ...values, state: resolution })
      message.success(resolution === 'accepted' ? '样本交接已接收并更新位置' : resolution === 'rejected' ? '交接已拒绝' : '交接已取消')
      setResolveTarget(null)
      resolveForm.resetFields()
      await refresh()
    } finally { setSaving(false) }
  }
  const showTimeline = async (row: CustodyTransfer) => setTimelineSpecimen(await specimenAPI.get(row.specimenId))
  const specimenLocation = (specimen: Specimen) => specimen.storageContainer
    ? [specimen.storageContainer.location, specimen.storageContainer.code, specimen.position].filter(Boolean).join(' / ')
    : 'intake'
  const columns: ColumnsType<CustodyTransfer> = [
    { title: '交接单号', dataIndex: 'transferNo', fixed: 'left' },
    { title: '样本', render: (_, row) => <div><strong>{row.specimen?.accessionNo || row.specimenId}</strong><small className="cell-subtitle">{row.specimen?.sampleType}</small></div> },
    { title: '状态', dataIndex: 'state', render: (value) => <CustodyBadge state={value} /> },
    { title: '交接人', render: (_, row) => `${row.fromCustodian} → ${row.toCustodian}` },
    { title: '位置变化', render: (_, row) => <div>{row.fromLocation}<small className="cell-subtitle">→ {row.toLocation}</small></div> },
    { title: '温度', dataIndex: 'temperatureC', render: (value) => value == null ? '-' : `${value} °C` },
    { title: '发起人/时间', render: (_, row) => <div>{row.preparedByName}<small className="cell-subtitle">{formatDateTime(row.preparedAt)}</small></div> },
    { title: '操作', fixed: 'right', render: (_, row) => <Space><Button size="small" icon={<HistoryOutlined />} onClick={() => void showTimeline(row)}>链路</Button>{row.state === 'prepared' && can('transfer:resolve') && <Button size="small" type="primary" icon={<CheckOutlined />} onClick={() => { setResolveTarget(row); setResolution('accepted') }}>处理</Button>}</Space> },
  ]
  return (
    <div className="page-stack">
      <header className="page-header"><div><Typography.Title level={2}>交接工作台</Typography.Title><Typography.Text type="secondary">双人确认样本责任与冻存位置，保留不可篡改的前后位置快照</Typography.Text></div>{can('transfer:prepare') && <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>发起交接</Button>}</header>
      <div className="table-toolbar"><Input allowClear prefix={<SearchOutlined />} placeholder="搜索交接单、样本或保管人" value={search} onChange={(event) => setSearch(event.target.value)} onPressEnter={() => void refresh()} /><Select allowClear placeholder="全部状态" value={state} onChange={setState} options={[{ value: 'prepared', label: '待接收' }, { value: 'accepted', label: '已接收' }, { value: 'rejected', label: '已拒绝' }, { value: 'cancelled', label: '已取消' }]} /><Button onClick={() => void refresh()}>查询</Button></div>
      <EntityTable columns={columns} dataSource={data.items} loading={loading} emptyTitle="暂无交接任务" pagination={{ current: pagination.page, pageSize: pagination.pageSize, total: data.total, onChange: pagination.update, showSizeChanger: true }} />
      <Modal title="发起样本交接" width={700} open={createOpen} confirmLoading={saving} onOk={() => void create()} onCancel={() => setCreateOpen(false)} okText="创建交接单" cancelText="取消">
        <Form form={createForm} layout="vertical" onValuesChange={(changed) => {
          if (!('specimenId' in changed)) return
          const specimen = specimens.find((item) => item.id === changed.specimenId)
          if (specimen) createForm.setFieldsValue({ fromCustodian: specimen.currentCustodian, fromLocation: specimenLocation(specimen) })
        }}><Row gutter={16}><Col span={12}><Form.Item name="transferNo" label="交接单号" rules={[{ required: true, min: 3 }]}><Input placeholder="TR-20260822-001" /></Form.Item></Col><Col span={12}><Form.Item name="specimenId" label="样本" rules={[{ required: true }]}><Select showSearch optionFilterProp="label" options={specimens.map((item) => ({ value: item.id, label: `${item.accessionNo} · ${item.sampleType}` }))} /></Form.Item></Col></Row><Row gutter={16}><Col span={12}><Form.Item name="fromCustodian" label="移交人" rules={[{ required: true }]}><Input /></Form.Item></Col><Col span={12}><Form.Item name="toCustodian" label="接收人" rules={[{ required: true }]}><Input /></Form.Item></Col></Row><Row gutter={16}><Col span={12}><Form.Item name="fromLocation" label="移交前位置" rules={[{ required: true }]}><Input /></Form.Item></Col><Col span={12}><Form.Item name="toLocation" label="目标位置描述" rules={[{ required: true }]}><Input /></Form.Item></Col></Row><Form.Item name="temperatureC" label="交接温度 (°C)"><InputNumber min={-200} max={30} precision={1} style={{ width: '100%' }} /></Form.Item><Form.Item name="reason" label="交接用途/备注"><Input.TextArea rows={3} maxLength={800} showCount /></Form.Item></Form>
      </Modal>
      <Modal title={`处理交接 ${resolveTarget?.transferNo || ''}`} width={620} open={Boolean(resolveTarget)} confirmLoading={saving} onOk={() => void resolve()} onCancel={() => setResolveTarget(null)} okText="确认处理" cancelText="返回">
        <Space direction="vertical" size="large" style={{ width: '100%' }}><Segmented block value={resolution} onChange={(value) => setResolution(value as Resolution)} options={[{ value: 'accepted', label: '接收', icon: <CheckOutlined /> }, { value: 'rejected', label: '拒绝', icon: <CloseOutlined /> }, { value: 'cancelled', label: '取消', icon: <StopOutlined /> }]} /><Form form={resolveForm} layout="vertical">{resolution === 'accepted' && <Row gutter={16}><Col span={12}><Form.Item name="toContainerId" label="目标冻存容器" rules={[{ required: true }]}><Select showSearch optionFilterProp="label" options={containers.map((item) => ({ value: item.id, label: `${item.code} · ${item.name}` }))} /></Form.Item></Col><Col span={12}><Form.Item name="toPosition" label="格位" rules={[{ required: true }]}><Input placeholder="R03-B05-C07" /></Form.Item></Col></Row>}<Form.Item name="temperatureC" label="交接复核温度 (°C)"><InputNumber min={-200} max={30} precision={1} style={{ width: '100%' }} /></Form.Item><Form.Item name="reason" label="处理说明" rules={[{ required: true, min: 3 }]}><Input.TextArea rows={3} /></Form.Item></Form></Space>
      </Modal>
      <Drawer width={660} title={timelineSpecimen ? `${timelineSpecimen.accessionNo} 交接链` : '交接链'} open={Boolean(timelineSpecimen)} onClose={() => setTimelineSpecimen(null)}><CustodyTimeline transfers={timelineSpecimen?.transfers || []} /></Drawer>
    </div>
  )
}
