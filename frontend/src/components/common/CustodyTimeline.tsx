import { ClockCircleOutlined, SwapOutlined } from '@ant-design/icons'
import { Empty, Timeline, Typography } from 'antd'
import type { CustodyTransfer } from '../../types/domain'
import { formatDateTime } from '../../utils/format'
import { CustodyBadge } from './CustodyBadge'

export function CustodyTimeline({ transfers }: { transfers: CustodyTransfer[] }) {
  if (!transfers.length) return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无交接记录" />
  return (
    <Timeline items={transfers.map((transfer) => ({
      color: transfer.state === 'accepted' ? 'green' : transfer.state === 'rejected' ? 'red' : 'blue',
      dot: transfer.state === 'prepared' ? <ClockCircleOutlined /> : <SwapOutlined />,
      children: <div className="timeline-item"><div><Typography.Text strong>{transfer.transferNo}</Typography.Text> <CustodyBadge state={transfer.state} /></div><Typography.Text>{transfer.fromCustodian} → {transfer.toCustodian}</Typography.Text><small>{transfer.fromLocation} → {transfer.toLocation} · {formatDateTime(transfer.preparedAt)}</small>{transfer.reason && <Typography.Paragraph type="secondary">{transfer.reason}</Typography.Paragraph>}</div>,
    }))} />
  )
}
