import { EyeOutlined, FileProtectOutlined, HistoryOutlined } from '@ant-design/icons'
import { Button, Checkbox, Col, DatePicker, Form, Input, Modal, Radio, Row, Segmented, Space, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useEffect, useState } from 'react'
import { protocolAPI, specimenAPI } from '../api'
import { CustodyBadge } from '../components/common/CustodyBadge'
import { EntityTable } from '../components/common/EntityTable'
import { SampleDrawer } from '../components/common/SampleDrawer'
import { StatusBadge } from '../components/common/StatusBadge'
import { useAuth } from '../hooks/useAuth'
import { useProtocolStore } from '../stores/protocolStore'
import type { ProtocolReview, ReviewDecision, Specimen } from '../types/domain'
import { formatDateTime } from '../utils/format'

export function ProtocolsPage() {
  const { can } = useAuth()
  const reviews = useProtocolStore()
  const [mode, setMode] = useState<'queue' | 'history'>('queue')
  const [specimens, setSpecimens] = useState<Specimen[]>([])
  const [loading, setLoading] = useState(false)
  const [reviewTarget, setReviewTarget] = useState<Specimen | null>(null)
  const [drawerSpecimen, setDrawerSpecimen] = useState<Specimen | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm()
  const load = async () => {
    setLoading(true)
    try {
      const result = await specimenAPI.list({ page: 1, pageSize: 100 })
      setSpecimens(result.items.filter((item) => !['disposed', 'released'].includes(item.state)))
      await reviews.load({ page: 1, pageSize: 100 })
    } finally { setLoading(false) }
  }
  useEffect(() => { void load() }, [])
  const show = async (specimen: Specimen) => setDrawerSpecimen(await specimenAPI.get(specimen.id))
  const submit = async () => {
    if (!reviewTarget) return
    const values = await form.validateFields()
    setSaving(true)
    try {
      await protocolAPI.create({
        specimenId: reviewTarget.id,
        protocolCode: reviewTarget.protocolCode,
        decision: values.decision as ReviewDecision,
        consentVerified: values.consentVerified,
        scopeVerified: values.scopeVerified,
        retentionUntil: values.retentionUntil?.toISOString(),
        documentObjectKey: values.documentObjectKey || '',
        notes: values.notes,
      })
      message.success('协议复核已记录')
      setReviewTarget(null)
      form.resetFields()
      await load()
    } finally { setSaving(false) }
  }
  const specimenColumns: ColumnsType<Specimen> = [
    { title: '样本', dataIndex: 'accessionNo', render: (value, row) => <Button type="link" className="table-link" onClick={() => void show(row)}>{value}</Button> },
    { title: '样本类型', dataIndex: 'sampleType' },
    { title: '来源协议', dataIndex: 'protocolCode' },
    { title: '保管状态', dataIndex: 'state', render: (value) => <CustodyBadge state={value} /> },
    { title: '已复核次数', render: (_, row) => row.protocolReviews?.length || 0 },
    { title: '操作', fixed: 'right', render: (_, row) => <Space><Button size="small" icon={<EyeOutlined />} onClick={() => void show(row)}>样本详情</Button>{can('protocol:review') && <Button size="small" type="primary" icon={<FileProtectOutlined />} onClick={() => { setReviewTarget(row); form.setFieldsValue({ decision: row.state === 'stored' ? 'approved' : 'hold', consentVerified: false, scopeVerified: false }) }}>复核</Button>}</Space> },
  ]
  const reviewColumns: ColumnsType<ProtocolReview> = [
    { title: '样本', render: (_, row) => row.specimen?.accessionNo || row.specimenId },
    { title: '协议', dataIndex: 'protocolCode' },
    { title: '决定', dataIndex: 'decision', render: (value) => <StatusBadge value={value} /> },
    { title: '知情同意', dataIndex: 'consentVerified', render: (value) => value ? '已核验' : '未核验' },
    { title: '使用范围', dataIndex: 'scopeVerified', render: (value) => value ? '符合' : '不符合' },
    { title: '复核人', dataIndex: 'reviewerName' },
    { title: '保留期限', dataIndex: 'retentionUntil', render: formatDateTime },
    { title: '复核时间', dataIndex: 'reviewedAt', render: formatDateTime },
    { title: '说明', dataIndex: 'notes', ellipsis: true },
  ]
  return (
    <div className="page-stack">
      <header className="page-header"><div><Typography.Title level={2}>协议复核</Typography.Title><Typography.Text type="secondary">核对知情同意、研究使用范围、样本保留期和协议文件</Typography.Text></div><Segmented value={mode} onChange={(value) => setMode(value as typeof mode)} options={[{ value: 'queue', label: '样本待复核', icon: <FileProtectOutlined /> }, { value: 'history', label: '复核记录', icon: <HistoryOutlined /> }]} /></header>
      {mode === 'queue' ? <EntityTable columns={specimenColumns} dataSource={specimens} loading={loading} pagination={{ pageSize: 10 }} emptyTitle="暂无可复核样本" /> : <EntityTable columns={reviewColumns} dataSource={reviews.data.items} loading={reviews.loading} pagination={{ pageSize: 10 }} emptyTitle="暂无协议复核记录" />}
      <Modal width={680} title={`协议复核 · ${reviewTarget?.accessionNo || ''}`} open={Boolean(reviewTarget)} confirmLoading={saving} onOk={() => void submit()} onCancel={() => setReviewTarget(null)} okText="提交复核" cancelText="取消">
        <Form form={form} layout="vertical"><Form.Item label="复核决定" name="decision" rules={[{ required: true }]}><Radio.Group optionType="button" buttonStyle="solid" options={[{ value: 'approved', label: '通过' }, { value: 'hold', label: '暂缓' }, { value: 'rejected', label: '拒绝' }]} /></Form.Item><Row gutter={16}><Col span={12}><Form.Item name="consentVerified" valuePropName="checked"><Checkbox>已核验有效知情同意</Checkbox></Form.Item></Col><Col span={12}><Form.Item name="scopeVerified" valuePropName="checked"><Checkbox>样本用途符合协议范围</Checkbox></Form.Item></Col></Row><Form.Item name="retentionUntil" label="样本保留期限"><DatePicker showTime style={{ width: '100%' }} /></Form.Item><Form.Item name="documentObjectKey" label="MinIO 协议文件对象键"><Input placeholder="protocols/PR-2026/consent-v2.pdf" /></Form.Item><Form.Item name="notes" label="复核说明" rules={[{ required: true, min: 5 }]}><Input.TextArea rows={4} maxLength={1000} showCount /></Form.Item></Form>
      </Modal>
      <SampleDrawer specimen={drawerSpecimen} open={Boolean(drawerSpecimen)} onClose={() => setDrawerSpecimen(null)} />
    </div>
  )
}
