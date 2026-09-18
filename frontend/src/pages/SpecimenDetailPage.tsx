import { ArrowLeftOutlined } from '@ant-design/icons'
import { Button, Descriptions, Spin, Typography } from 'antd'
import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { specimenAPI } from '../api'
import { CustodyBadge } from '../components/common/CustodyBadge'
import { CustodyTimeline } from '../components/common/CustodyTimeline'
import { EntityTable } from '../components/common/EntityTable'
import { StatusBadge } from '../components/common/StatusBadge'
import type { ProtocolReview, Specimen } from '../types/domain'
import { formatDateTime } from '../utils/format'

export function SpecimenDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const [specimen, setSpecimen] = useState<Specimen | null>(null)
  const [loading, setLoading] = useState(true)
  useEffect(() => { void specimenAPI.get(Number(id)).then(setSpecimen).finally(() => setLoading(false)) }, [id])
  if (loading || !specimen) return <Spin fullscreen />
  return (
    <div className="page-stack">
      <header className="page-header"><div><Button type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/specimens')}>返回样本队列</Button><Typography.Title level={2}>{specimen.accessionNo}</Typography.Title></div><CustodyBadge state={specimen.state} /></header>
      <section className="detail-section"><Typography.Title level={4}>样本信息</Typography.Title><Descriptions bordered size="small" column={{ xs: 1, md: 3 }}><Descriptions.Item label="类型">{specimen.sampleType}</Descriptions.Item><Descriptions.Item label="受试者编码">{specimen.subjectCode}</Descriptions.Item><Descriptions.Item label="来源协议">{specimen.protocolCode}</Descriptions.Item><Descriptions.Item label="保管人">{specimen.currentCustodian}</Descriptions.Item><Descriptions.Item label="体积/分装">{specimen.volumeMl} mL / {specimen.aliquotCount} 份</Descriptions.Item><Descriptions.Item label="接收时间">{formatDateTime(specimen.receivedAt)}</Descriptions.Item><Descriptions.Item label="冻存容器">{specimen.storageContainer?.name || '待分配'}</Descriptions.Item><Descriptions.Item label="格位">{specimen.position || '-'}</Descriptions.Item><Descriptions.Item label="温区">{specimen.storageContainer ? <StatusBadge value={specimen.storageContainer.temperatureZone} /> : '-'}</Descriptions.Item></Descriptions></section>
      <section className="detail-section"><Typography.Title level={4}>交接链</Typography.Title><CustodyTimeline transfers={specimen.transfers || []} /></section>
      <section className="detail-section"><Typography.Title level={4}>协议复核</Typography.Title><EntityTable<ProtocolReview> pagination={false} dataSource={specimen.protocolReviews || []} columns={[{ title: '协议', dataIndex: 'protocolCode' }, { title: '决定', dataIndex: 'decision', render: (value) => <StatusBadge value={value} /> }, { title: '复核人', dataIndex: 'reviewerName' }, { title: '复核时间', dataIndex: 'reviewedAt', render: formatDateTime }, { title: '说明', dataIndex: 'notes' }]} /></section>
    </div>
  )
}
