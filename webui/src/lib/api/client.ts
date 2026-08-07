import axios from 'axios'
import { sessionExpired } from '@/lib/authState'

const API_BASE_URL = import.meta.env.VITE_SERVER_URL_V1 || '/api/v1'

export const apiClient = axios.create({
    baseURL: API_BASE_URL,
    headers: {
        'Content-Type': 'application/json'
    },
    // The session cookie must ride along when the Vite dev server points at a
    // different origin (VITE_SERVER_URL_V1); same-origin production is unaffected.
    withCredentials: true
})

// A 401 from any /api/v1 call means the session is gone (expired, or the
// server restarted with fresh cookie keys) — flag it so the SPA renders the
// login view. The login endpoint itself is exempt: its 401 is the uniform
// wrong-credentials answer, not a lost session.
apiClient.interceptors.response.use(
    (response) => response,
    (error) => {
        const status = error?.response?.status
        const url: string = error?.config?.url ?? ''
        if (status === 401 && !url.includes('/auth/login')) {
            sessionExpired.value = true
        }
        return Promise.reject(error)
    }
)
