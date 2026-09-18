import { useCallback, useState } from 'react'
import { transferAPI, type PageParams } from '../api'
import type { CustodyTransfer, PageResult, TransferState } from '../types/domain'

export function useTransferStore() {
  const [data, setData] = useState<PageResult<CustodyTransfer>>({ items: [], total: 0, page: 1, pageSize: 10 })
  const [loading, setLoading] = useState(false)
  const load = useCallback(async (params: PageParams & { state?: TransferState; specimenId?: number } = {}) => {
    setLoading(true)
    try { setData(await transferAPI.list(params)) } finally { setLoading(false) }
  }, [])
  return { data, loading, load }
}
