import type { User } from '../types/domain'

export const can = (user: User | null, permission: string) => Boolean(user?.permissions.includes(permission))

export const roleLabels = {
  admin: '样本库管理员', receiver: '接收专员', custodian: '保管员', reviewer: '协议复核员', auditor: '审计员',
} as const
