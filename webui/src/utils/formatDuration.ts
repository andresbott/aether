/**
 * Format a duration in seconds as "m:ss" (or "h:mm:ss" when >= 1 hour).
 * Returns an empty string for missing, zero, or negative values.
 */
export function formatDuration(seconds?: number): string {
    if (!seconds || seconds < 0) return ''
    const total = Math.floor(seconds)
    const h = Math.floor(total / 3600)
    const m = Math.floor((total % 3600) / 60)
    const s = total % 60
    const ss = s.toString().padStart(2, '0')
    if (h > 0) {
        return `${h}:${m.toString().padStart(2, '0')}:${ss}`
    }
    return `${m}:${ss}`
}
