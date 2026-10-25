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

/**
 * Format a total running time in words: "43 min", "2 hr 10 min", "1 hr", or
 * "32 sec" under a minute — how a summary or a meta line reads a length, where
 * a clock's seconds would be noise. Returns an empty string for missing, zero,
 * or negative values.
 */
export function formatTotalDuration(seconds?: number): string {
    if (!seconds || seconds < 0) return ''
    const total = Math.floor(seconds)
    if (total < 60) return `${total} sec`
    const h = Math.floor(total / 3600)
    const m = Math.floor((total % 3600) / 60)
    if (h === 0) return `${m} min`
    return m > 0 ? `${h} hr ${m} min` : `${h} hr`
}
