export type SpecimenState = 'received' | 'aliquoted' | 'stored' | 'released' | 'disposed'
export type TransferState = 'prepared' | 'accepted' | 'rejected' | 'cancelled'
export type ReviewDecision = 'approved' | 'hold' | 'rejected'
export type TemperatureZone = 'minus20' | 'minus80' | 'liquid_nitrogen'
export type Role = 'admin' | 'receiver' | 'custodian' | 'reviewer' | 'auditor'

export interface BaseEntity {
  id: number
  createdAt: string
  updatedAt: string
}

export interface StorageContainer extends BaseEntity {
  code: string
  name: string
  containerType: string
  temperatureZone: TemperatureZone
  location: string
  capacity: number
  occupied: number
  status: 'available' | 'maintenance' | 'alarm'
  active: boolean
  specimens?: Specimen[]
}

export interface Specimen extends BaseEntity {
  accessionNo: string
  sampleType: string
  subjectCode: string
  protocolCode: string
  state: SpecimenState
  storageContainerId?: number
  storageContainer?: StorageContainer
  position?: string
  volumeMl: number
  aliquotCount: number
  currentCustodian: string
  receivedAt: string
  expiresAt?: string
  notes?: string
  transfers?: CustodyTransfer[]
  protocolReviews?: ProtocolReview[]
}

export interface CustodyTransfer extends BaseEntity {
  specimenId: number
  specimen?: Specimen
  transferNo: string
  fromCustodian: string
  toCustodian: string
  fromLocation: string
  toLocation: string
  toContainerId?: number
  toContainer?: StorageContainer
  toPosition?: string
  state: TransferState
  preparedById: number
  preparedByName: string
  acceptedById?: number
  acceptedByName?: string
  preparedAt: string
  resolvedAt?: string
  temperatureC?: number
  reason?: string
}

export interface ProtocolReview extends BaseEntity {
  specimenId: number
  specimen?: Specimen
  protocolCode: string
  decision: ReviewDecision
  reviewerId: number
  reviewerName: string
  consentVerified: boolean
  scopeVerified: boolean
  retentionUntil?: string
  documentObjectKey?: string
  notes: string
  reviewedAt: string
}

export interface AuditLog {
  id: number
  createdAt: string
  requestId: string
  actorId: number
  actorName: string
  action: string
  entityType: string
  entityId: number
  beforeState: string
  afterState: string
  beforeLocation: string
  afterLocation: string
  beforeCustodian: string
  afterCustodian: string
  ipAddress: string
  previousHash: string
  entryHash: string
}

export interface User {
  id: number
  username: string
  displayName: string
  role: Role
  permissions: string[]
}

export interface PageResult<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}

export interface APIEnvelope<T> { data: T; requestId: string }
