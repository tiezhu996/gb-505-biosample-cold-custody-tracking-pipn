import { EyeOutlined, SearchOutlined } from '@ant-design/icons'
import { Button, Descriptions, Drawer, Input, Select, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useEffect, useState } from 'react'
import { auditAPI } from '../api'
import { EntityTable } from '../components/common/EntityTable'
import { usePagination } from '../hooks/usePagination'
import type { AuditLog, PageResult } from '../types/domain'
import { actionLabels, formatDateTime } from '../utils/format'

const empty: PageResult<AuditLog> = { items: [], total: 0, page: 1, pageSize: 10 }
export function AuditPage() {
  const pagination = usePagination()
  const [data, setData] = useState(empty)
  const [loading, setLoading] = useState(false)
  const [search, setSearch] = useState('')
  const [entityType, setEntityType] = useState<string>()
  const [selected, setSelected] = useState<AuditLog | null>(null)
  const load = async () => { setLoading(true); try { setData(await auditAPI.list({ page: pagination.page, pageSize: pagination.pageSize, search, entityType })) } finally { setLoading(false) } }
  useEffect(() => { void load() }, [pagination.page, pagination.pageSize, entityType])
  const pretty = (value: string) => { try { return JSON.stringify(JSON.parse(value), null, 2) } catch { return value || '-' } }
  const columns: ColumnsType<AuditLog> = [
    { title: '发生时间', dataIndex: 'createdAt', render: formatDateTime },
    { title: '操作者', dataIndex: 'actorName' },
    { title: '操作', dataIndex: 'action', render: (value) => actionLabels[value] || value },
    { title: '实体', render: (_, row) => `${row.entityType} #${row.entityId}` },
    { title: '请求 ID', dataIndex: 'requestId', ellipsis: true, width: 210 },
    { title: '来源 IP', dataIndex: 'ipAddress' },
    { title: '详情', fixed: 'right', render: (_, row) => <Button size="small" icon={<EyeOutlined />} onClick={() => setSelected(row)}>查看</Button> },
  ]
  return (
    <div className="page-stack">
      <header className="page-header"><div><Typography.Title level={2}>链路审计</Typography.Title><Typography.Text type="secondary">追踪样本责任、前后冻存位置、操作者和请求 ID；审计事件只追加不可修改</Typography.Text></div></header>
      <div className="table-toolbar"><Input allowClear prefix={<SearchOutlined />} placeholder="搜索操作、人员或请求 ID" value={search} onChange={(event) => setSearch(event.target.value)} onPressEnter={() => void load()} /><Select allowClear placeholder="全部实体" value={entityType} onChange={setEntityType} options={[{ value: 'StorageContainer', label: '冻存容器' }, { value: 'Specimen', label: '样本' }, { value: 'CustodyTransfer', label: '交接记录' }, { value: 'ProtocolReview', label: '协议复核' }]} /><Button onClick={() => void load()}>查询</Button></div>
      <EntityTable columns={columns} dataSource={data.items} loading={loading} emptyTitle="暂无审计记录" pagination={{ current: pagination.page, pageSize: pagination.pageSize, total: data.total, onChange: pagination.update, showSizeChanger: true }} />
      <Drawer title="审计详情" width={680} open={Boolean(selected)} onClose={() => setSelected(null)}><Descriptions column={1} size="small" bordered><Descriptions.Item label="操作">{selected ? actionLabels[selected.action] || selected.action : ''}</Descriptions.Item><Descriptions.Item label="操作者">{selected?.actorName}</Descriptions.Item><Descriptions.Item label="请求 ID"><Typography.Text copyable>{selected?.requestId}</Typography.Text></Descriptions.Item><Descriptions.Item label="位置变化">{selected?.beforeLocation || '-'} → {selected?.afterLocation || '-'}</Descriptions.Item><Descriptions.Item label="保管人变化">{selected?.beforeCustodian || '-'} → {selected?.afterCustodian || '-'}</Descriptions.Item><Descriptions.Item label="审计哈希"><Typography.Text copyable className="hash-value">{selected?.entryHash}</Typography.Text></Descriptions.Item><Descriptions.Item label="变更前"><pre className="json-state">{selected ? pretty(selected.beforeState) : ''}</pre></Descriptions.Item><Descriptions.Item label="变更后"><pre className="json-state">{selected ? pretty(selected.afterState) : ''}</pre></Descriptions.Item></Descriptions></Drawer>
    </div>
  )
}
