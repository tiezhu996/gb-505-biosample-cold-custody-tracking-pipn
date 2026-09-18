import { useCallback, useState } from 'react'
import { storageAPI, type PageParams } from '../api'
import type { PageResult, StorageContainer } from '../types/domain'

export function useStorageStore() {
  const [data, setData] = useState<PageResult<StorageContainer>>({ items: [], total: 0, page: 1, pageSize: 10 })
  const [loading, setLoading] = useState(false)
  const load = useCallback(async (params: PageParams & { temperatureZone?: string; status?: string } = {}) => {
    setLoading(true)
    try { setData(await storageAPI.list(params)) } finally { setLoading(false) }
  }, [])
  return { data, loading, load }
}
