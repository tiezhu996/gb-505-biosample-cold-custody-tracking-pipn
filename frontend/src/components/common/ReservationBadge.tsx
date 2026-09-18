import { Tag } from 'antd'
import type { ReservationStatus } from '../../types/domain'

const reservationStates: Record<Exclude<ReservationStatus, ''>, { label: string; color: string }> = {
  active: { label: '预约中', color: 'processing' },
  consumed: { label: '已转占用', color: 'success' },
  released: { label: '已释放', color: 'default' },
  expired: { label: '已到期', color: 'warning' },
}

export function ReservationBadge({ status }: { status?: ReservationStatus }) {
  if (!status) return null
  const item = reservationStates[status]
  return <Tag color={item?.color || 'default'}>{item?.label || status}</Tag>
}
