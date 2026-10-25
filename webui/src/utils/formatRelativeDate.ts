const DAY_MS = 24 * 60 * 60 * 1000

const startOfDay = (d: Date): number =>
    new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()

/**
 * How long ago an ISO date was, counted in calendar days: "today", "yesterday",
 * "3 days ago", "2 weeks ago", "5 months ago", "a year ago". Returns an empty
 * string for a missing or unparseable date; a future date (clock skew) reads as
 * today.
 */
export function formatRelativeDate(iso?: string, now: Date = new Date()): string {
    if (!iso) return ''
    const date = new Date(iso)
    if (Number.isNaN(date.getTime())) return ''
    // Rounded, because a day across a DST change is 23 or 25 hours long.
    const days = Math.round((startOfDay(now) - startOfDay(date)) / DAY_MS)
    if (days <= 0) return 'today'
    if (days === 1) return 'yesterday'
    if (days < 14) return `${days} days ago`
    // Weeks until a count of months would start at two, never "1 months".
    if (days < 61) return `${Math.floor(days / 7)} weeks ago`
    if (days < 365) return `${Math.floor(days / 30.4)} months ago`
    const years = Math.floor(days / 365)
    return years === 1 ? 'a year ago' : `${years} years ago`
}
