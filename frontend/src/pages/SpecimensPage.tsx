import { EyeOutlined, ForkOutlined, PlusOutlined, SearchOutlined } from '@ant-design/icons'
import { Button, Col, Form, Input, InputNumber, Modal, Row, Select, Space, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useEffect, useState } from 'react'
import { specimenAPI } from '../api'
import { CustodyBadge } from '../components/common/CustodyBadge'
import { EntityTable } from '../components/common/EntityTable'
import { SampleDrawer } from '../components/common/SampleDrawer'
import { useAuth } from '../hooks/useAuth'
import { usePagination } from '../hooks/usePagination'
import { useSpecimenStore } from '../stores/specimenStore'
import type { Specimen, SpecimenState } from '../types/domain'
import { formatDateTime } from '../utils/format'

export function SpecimensPage() {
  const { data, loading, load } = useSpecimenStore()
  const pagination = usePagination()
  const { can } = useAuth()
  const [search, setSearch] = useState('')
  const [state, setState] = useState<SpecimenState>()
  const [open, setOpen] = useState(false)
  const [aliquotTarget, setAliquotTarget] = useState<Specimen | null>(null)
  const [saving, setSaving] = useState(false)
  const [selected, setSelected] = useState<Specimen | null>(null)
  const [form] = Form.useForm()
  const [aliquotForm] = Form.useForm()
  const refresh = () => load({ page: pagination.page, pageSize: pagination.pageSize, search, state })
  useEffect(() => { void refresh() }, [pagination.page, pagination.pageSize, state])

  const create = async () => {
    const values = await form.validateFields()
    setSaving(true)
    try {
      await specimenAPI.create({ ...values, notes: values.notes || '' })
      message.success('样本接收登记成功')
      setOpen(false)
      form.resetFields()
      await refresh()
    } finally { setSaving(false) }
  }
  const show = async (specimen: Specimen) => setSelected(await specimenAPI.get(specimen.id))
  const markAliquoted = async () => {
    if (!aliquotTarget) return
    const { aliquotCount } = await aliquotForm.validateFields()
    setSaving(true)
    try {
      if (aliquotCount !== aliquotTarget.aliquotCount) await specimenAPI.update(aliquotTarget.id, { aliquotCount })
      await specimenAPI.transition(aliquotTarget.id, 'aliquoted', '完成分装并核对标签')
      message.success('样本已标记为分装完成')
      setAliquotTarget(null)
      aliquotForm.resetFields()
      await refresh()
    } finally { setSaving(false) }
  }
  const columns: ColumnsType<Specimen> = [
    { title: '样本接收号', dataIndex: 'accessionNo', fixed: 'left', render: (value, row) => <Button type="link" className="table-link" onClick={() => void show(row)}>{value}</Button> },
    { title: '样本类型', dataIndex: 'sampleType' },
    { title: '受试者编码', dataIndex: 'subjectCode' },
    { title: '协议', dataIndex: 'protocolCode' },
    { title: '状态', dataIndex: 'state', render: (value) => <CustodyBadge state={value} /> },
    { title: '冻存位置', render: (_, row) => row.storageContainer ? `${row.storageContainer.code} / ${row.position || '-'}` : '待分配' },
    { title: '当前保管人', dataIndex: 'currentCustodian' },
    { title: '体积/分装', render: (_, row) => `${row.volumeMl} mL / ${row.aliquotCount} 份` },
    { title: '接收时间', dataIndex: 'receivedAt', render: formatDateTime },
    { title: '操作', fixed: 'right', render: (_, row) => <Space><Button size="small" icon={<EyeOutlined />} onClick={() => void show(row)}>详情</Button>{row.state === 'received' && can('specimen:transition') && <Button size="small" icon={<ForkOutlined />} onClick={() => { setAliquotTarget(row); aliquotForm.setFieldsValue({ aliquotCount: Math.max(row.aliquotCount, 1) }) }}>完成分装</Button>}</Space> },
  ]
  return (
    <div className="page-stack">
      <header className="page-header"><div><Typography.Title level={2}>样本队列</Typography.Title><Typography.Text type="secondary">接收科研样本并追踪冻存状态、责任人和来源协议</Typography.Text></div>{can('specimen:create') && <Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>接收样本</Button>}</header>
      <div className="table-toolbar"><Input allowClear prefix={<SearchOutlined />} placeholder="搜索接收号、受试者编码或协议" value={search} onChange={(event) => setSearch(event.target.value)} onPressEnter={() => void refresh()} /><Select allowClear placeholder="全部状态" value={state} onChange={setState} options={[{ value: 'received', label: '已接收' }, { value: 'aliquoted', label: '已分装' }, { value: 'stored', label: '已冻存' }, { value: 'released', label: '已出库' }, { value: 'disposed', label: '已处置' }]} /><Button onClick={() => void refresh()}>查询</Button></div>
      <EntityTable columns={columns} dataSource={data.items} loading={loading} emptyTitle="暂无样本" emptyActionLabel={can('specimen:create') ? '接收首个样本' : undefined} onEmptyAction={() => setOpen(true)} pagination={{ current: pagination.page, pageSize: pagination.pageSize, total: data.total, showSizeChanger: true, onChange: pagination.update }} />
      <Modal width={680} title="接收新样本" open={open} confirmLoading={saving} onOk={() => void create()} onCancel={() => setOpen(false)} okText="确认接收" cancelText="取消">
        <Form form={form} layout="vertical" initialValues={{ aliquotCount: 1 }}>
          <Row gutter={16}><Col span={12}><Form.Item name="accessionNo" label="样本接收号" rules={[{ required: true, min: 3 }]}><Input placeholder="SP-20260822-001" /></Form.Item></Col><Col span={12}><Form.Item name="sampleType" label="样本类型" rules={[{ required: true }]}><Select options={[{ value: 'plasma', label: '血浆' }, { value: 'serum', label: '血清' }, { value: 'whole_blood', label: '全血' }, { value: 'tissue', label: '组织' }, { value: 'dna', label: 'DNA' }, { value: 'rna', label: 'RNA' }]} /></Form.Item></Col></Row>
          <Row gutter={16}><Col span={12}><Form.Item name="subjectCode" label="受试者脱敏编码" rules={[{ required: true, min: 3 }]}><Input placeholder="SUBJ-A032" /></Form.Item></Col><Col span={12}><Form.Item name="protocolCode" label="研究协议编号" rules={[{ required: true, min: 3 }]}><Input placeholder="PR-ONCO-2026-08" /></Form.Item></Col></Row>
          <Row gutter={16}><Col span={8}><Form.Item name="volumeMl" label="体积 (mL)" rules={[{ required: true }]}><InputNumber min={0.01} precision={2} style={{ width: '100%' }} /></Form.Item></Col><Col span={8}><Form.Item name="aliquotCount" label="分装份数" rules={[{ required: true }]}><InputNumber min={1} precision={0} style={{ width: '100%' }} /></Form.Item></Col><Col span={8}><Form.Item name="currentCustodian" label="接收保管人" rules={[{ required: true }]}><Input /></Form.Item></Col></Row>
          <Form.Item name="notes" label="接收备注"><Input.TextArea rows={3} maxLength={1000} showCount /></Form.Item>
        </Form>
      </Modal>
      <Modal title={`完成分装 · ${aliquotTarget?.accessionNo || ''}`} open={Boolean(aliquotTarget)} confirmLoading={saving} onOk={() => void markAliquoted()} onCancel={() => setAliquotTarget(null)} okText="确认完成" cancelText="取消">
        <Form form={aliquotForm} layout="vertical"><Form.Item name="aliquotCount" label="实际分装份数" rules={[{ required: true }]}><InputNumber min={1} max={10000} precision={0} style={{ width: '100%' }} /></Form.Item></Form>
      </Modal>
      <SampleDrawer specimen={selected} open={Boolean(selected)} onClose={() => setSelected(null)} />
    </div>
  )
}
