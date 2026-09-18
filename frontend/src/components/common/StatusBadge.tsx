import { Badge, Tag } from 'antd'

const colors: Record<string, string> = {
  available: 'success', maintenance: 'warning', alarm: 'error',
  approved: 'success', hold: 'warning', rejected: 'error',
  minus20: 'cyan', minus80: 'blue', liquid_nitrogen: 'purple',
}
const labels: Record<string, string> = {
  available: '可用', maintenance: '维护中', alarm: '温度告警',
  approved: '通过', hold: '暂缓', rejected: '拒绝',
  minus20: '-20°C', minus80: '-80°C', liquid_nitrogen: '液氮区',
}

export function StatusBadge({ value, dot = false }: { value: string; dot?: boolean }) {
  const label = labels[value] || value
  if (dot) return <Badge status={colors[value] as 'success' | 'warning' | 'error' | 'processing' | 'default'} text={label} />
  return <Tag color={colors[value]}>{label}</Tag>
}
