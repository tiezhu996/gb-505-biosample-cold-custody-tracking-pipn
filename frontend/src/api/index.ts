import { apiClient, unwrap } from './client'
import type {
  AuditLog, CustodyTransfer, PageResult, ProtocolReview, ReviewDecision,
  Specimen, SpecimenState, StorageContainer, TransferState, User,
} from '../types/domain'

export interface PageParams { page?: number; pageSize?: number; search?: string }

export const authAPI = {
  login: (username: string, password: string) => unwrap<{ token: string; expiresAt: number; user: User }>(
    apiClient.post('/auth/login', { username, password }),
  ),
  me: () => unwrap<User>(apiClient.get('/auth/me')),
}

export const storageAPI = {
  list: (params: PageParams & { temperatureZone?: string; status?: string } = {}) =>
    unwrap<PageResult<StorageContainer>>(apiClient.get('/storage-containers', { params })),
  get: (id: number) => unwrap<StorageContainer>(apiClient.get(`/storage-containers/${id}`)),
  create: (payload: Omit<StorageContainer, 'id' | 'createdAt' | 'updatedAt' | 'occupied' | 'active' | 'specimens'>) =>
    unwrap<StorageContainer>(apiClient.post('/storage-containers', payload)),
  update: (id: number, payload: Partial<StorageContainer>) =>
    unwrap<StorageContainer>(apiClient.patch(`/storage-containers/${id}`, payload)),
}

export const specimenAPI = {
  list: (params: PageParams & { state?: SpecimenState; storageContainerId?: number } = {}) =>
    unwrap<PageResult<Specimen>>(apiClient.get('/specimens', { params })),
  get: (id: number) => unwrap<Specimen>(apiClient.get(`/specimens/${id}`)),
  create: (payload: {
    accessionNo: string; sampleType: string; subjectCode: string; protocolCode: string;
    volumeMl: number; aliquotCount: number; currentCustodian: string; receivedAt?: string; notes?: string;
  }) => unwrap<Specimen>(apiClient.post('/specimens', payload)),
  update: (id: number, payload: Partial<Specimen>) => unwrap<Specimen>(apiClient.patch(`/specimens/${id}`, payload)),
  transition: (id: number, state: SpecimenState, reason = '') =>
    unwrap<Specimen>(apiClient.post(`/specimens/${id}/transition`, { state, reason })),
}

export const transferAPI = {
  list: (params: PageParams & { state?: TransferState; specimenId?: number } = {}) =>
    unwrap<PageResult<CustodyTransfer>>(apiClient.get('/custody-transfers', { params })),
  get: (id: number) => unwrap<CustodyTransfer>(apiClient.get(`/custody-transfers/${id}`)),
  create: (payload: {
    specimenId: number; transferNo: string; fromCustodian: string; toCustodian: string;
    fromLocation: string; toLocation: string; temperatureC?: number; reason?: string;
  }) => unwrap<CustodyTransfer>(apiClient.post('/custody-transfers', payload)),
  resolve: (id: number, payload: {
    state: Extract<TransferState, 'accepted' | 'rejected' | 'cancelled'>;
    toContainerId?: number; toPosition?: string; temperatureC?: number; reason: string;
  }) => unwrap<CustodyTransfer>(apiClient.post(`/custody-transfers/${id}/resolve`, payload)),
}

export const protocolAPI = {
  list: (params: PageParams & { decision?: ReviewDecision; specimenId?: number } = {}) =>
    unwrap<PageResult<ProtocolReview>>(apiClient.get('/protocol-reviews', { params })),
  create: (payload: {
    specimenId: number; protocolCode: string; decision: ReviewDecision; consentVerified: boolean;
    scopeVerified: boolean; retentionUntil?: string; documentObjectKey?: string; notes: string;
  }) => unwrap<ProtocolReview>(apiClient.post('/protocol-reviews', payload)),
}

export const auditAPI = {
  list: (params: PageParams & { entityType?: string; actorId?: number } = {}) =>
    unwrap<PageResult<AuditLog>>(apiClient.get('/audit-logs', { params })),
}
