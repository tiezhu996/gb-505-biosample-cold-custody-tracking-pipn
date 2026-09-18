import { Tag } from 'antd'
import type { ReservationState, SpecimenState, TransferState } from '../../types/domain'

const specimenStates: Record<SpecimenState, { label: string; color: string }> = {
  received: { label: '已接收', color: 'cyan' },
  aliquoted: { label: '已分装', color: 'blue' },
  stored: { label: '已冻存', color: 'success' },
  released: { label: '已出库', color: 'warning' },
  disposed: { label: '已处置', color: 'default' },
}
const transferStates: Record<TransferState, { label: string; color: string }> = {
  prepared: { label: '待接收', color: 'processing' },
  accepted: { label: '已接收', color: 'success' },
  rejected: { label: '已拒绝', color: 'error' },
  cancelled: { label: '已取消', color: 'default' },
}
const reservationStates: Record<ReservationState | 'none', { label: string; color: string }> = {
  active: { label: '预约中', color: 'processing' },
  consumed: { label: '已转占用', color: 'success' },
  released: { label: '已释放', color: 'default' },
  expired: { label: '已过期', color: 'warning' },
  none: { label: '未预约', color: 'default' },
}

export function CustodyBadge({ state }: { state: SpecimenState | TransferState }) {
  const item = specimenStates[state as SpecimenState] || transferStates[state as TransferState]
  return <Tag color={item?.color || 'default'}>{item?.label || state}</Tag>
}

export function ReservationBadge({ state }: { state?: ReservationState | 'none' }) {
  const item = reservationStates[state || 'none'] || reservationStates.none
  return <Tag color={item.color}>{item.label}</Tag>
}
