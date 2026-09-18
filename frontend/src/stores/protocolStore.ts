import { useCallback, useState } from 'react'
import { protocolAPI, type PageParams } from '../api'
import type { PageResult, ProtocolReview, ReviewDecision } from '../types/domain'

export function useProtocolStore() {
  const [data, setData] = useState<PageResult<ProtocolReview>>({ items: [], total: 0, page: 1, pageSize: 10 })
  const [loading, setLoading] = useState(false)
  const load = useCallback(async (params: PageParams & { decision?: ReviewDecision; specimenId?: number } = {}) => {
    setLoading(true)
    try { setData(await protocolAPI.list(params)) } finally { setLoading(false) }
  }, [])
  return { data, loading, load }
}
