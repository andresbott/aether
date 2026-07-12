import { apiClient } from '@/lib/api/client'
import type { RadioBrowserStation } from '@/types/radiobrowser'

export async function searchRadioStations(query: string): Promise<RadioBrowserStation[]> {
    const { data } = await apiClient.get<RadioBrowserStation[]>('/radiobrowser/search', {
        params: { q: query }
    })
    return data
}

// fetchRadioFavicon downloads a station favicon through the backend proxy and
// wraps it as a File suitable for the cover upload. Best-effort: returns null on
// any failure (missing, broken, or unsupported favicon) so the caller simply
// skips the cover instead of failing the whole import.
export async function fetchRadioFavicon(faviconUrl: string): Promise<File | null> {
    if (!faviconUrl) return null
    try {
        const resp = await apiClient.get('/radiobrowser/favicon', {
            params: { url: faviconUrl },
            responseType: 'blob',
            // Fetched in the background while the form is already shown; keep it
            // short so a slow favicon host never leaves the cover hanging.
            timeout: 8000
        })
        const blob = resp.data as Blob
        // The proxy only ever returns PNG or JPEG; blob.type carries which.
        const type = blob.type || 'image/png'
        const ext = type === 'image/jpeg' ? 'jpg' : 'png'
        return new File([blob], `favicon.${ext}`, { type })
    } catch {
        return null
    }
}
