import { useCallback, useState } from 'react'
import { specimenAPI, type PageParams } from '../api'
import type { PageResult, Specimen, SpecimenState } from '../types/domain'

export function useSpecimenStore() {
  const [data, setData] = useState<PageResult<Specimen>>({ items: [], total: 0, page: 1, pageSize: 10 })
  const [loading, setLoading] = useState(false)
  const load = useCallback(async (params: PageParams & { state?: SpecimenState; storageContainerId?: number } = {}) => {
    setLoading(true)
    try { setData(await specimenAPI.list(params)) } finally { setLoading(false) }
  }, [])
  return { data, loading, load }
}
