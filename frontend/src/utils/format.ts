import dayjs from 'dayjs'

export const formatDateTime = (value?: string) => value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '-'
export const formatNumber = (value?: number) => new Intl.NumberFormat('zh-CN').format(value ?? 0)

export const actionLabels: Record<string, string> = {
  'storage_container.created': '创建冻存容器', 'storage_container.updated': '更新冻存容器',
  'specimen.received': '接收样本', 'specimen.updated': '更新样本',
  'specimen.transitioned': '变更样本状态', 'specimen.relocated': '更新样本位置',
  'specimen.released': '协议批准放行样本',
  'custody_transfer.prepared': '发起交接', 'custody_transfer.accepted': '接收交接',
  'custody_transfer.rejected': '拒绝交接', 'custody_transfer.cancelled': '取消交接',
  'protocol_review.created': '完成协议复核',
}

export const temperatureLabels: Record<string, string> = {
  minus20: '-20°C 冷冻', minus80: '-80°C 超低温', liquid_nitrogen: '液氮气相',
}
