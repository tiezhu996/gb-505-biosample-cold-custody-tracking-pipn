import { Tag } from 'antd'
import type { SpecimenState, TransferState } from '../../types/domain'

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

export function CustodyBadge({ state }: { state: SpecimenState | TransferState }) {
  const item = specimenStates[state as SpecimenState] || transferStates[state as TransferState]
  return <Tag color={item?.color || 'default'}>{item?.label || state}</Tag>
}
