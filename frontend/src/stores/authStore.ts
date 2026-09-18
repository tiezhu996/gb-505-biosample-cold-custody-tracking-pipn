import { useSyncExternalStore } from 'react'
import { authAPI } from '../api'
import type { User } from '../types/domain'

interface AuthState { user: User | null; loading: boolean }

let state: AuthState = {
  user: JSON.parse(localStorage.getItem('biosample.user') || 'null') as User | null,
  loading: false,
}
const listeners = new Set<() => void>()
const emit = () => listeners.forEach((listener) => listener())
const setState = (next: Partial<AuthState>) => { state = { ...state, ...next }; emit() }

export const authStore = {
  subscribe: (listener: () => void) => { listeners.add(listener); return () => listeners.delete(listener) },
  getSnapshot: () => state,
  async login(username: string, password: string) {
    setState({ loading: true })
    try {
      const result = await authAPI.login(username, password)
      localStorage.setItem('biosample.accessToken', result.token)
      localStorage.setItem('biosample.user', JSON.stringify(result.user))
      setState({ user: result.user })
    } finally { setState({ loading: false }) }
  },
  async refresh() {
    if (!localStorage.getItem('biosample.accessToken')) return
    const user = await authAPI.me()
    localStorage.setItem('biosample.user', JSON.stringify(user))
    setState({ user })
  },
  logout() {
    localStorage.removeItem('biosample.accessToken')
    localStorage.removeItem('biosample.user')
    setState({ user: null })
  },
}

export const useAuthState = () => useSyncExternalStore(authStore.subscribe, authStore.getSnapshot)
