import type { ImageMeta } from '@/types/metadata'

// formatBytes renders a byte count as a compact human size (B / KB / MB / …),
// with a single decimal for fractional values and none for whole ones —
// "512 B", "1.5 KB", "245 KB", "3 MB".
export function formatBytes(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`
    const units = ['KB', 'MB', 'GB', 'TB']
    let value = bytes / 1024
    let unit = 0
    while (value >= 1024 && unit < units.length - 1) {
        value /= 1024
        unit++
    }
    const rounded = Math.round(value * 10) / 10
    const text = Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1)
    return `${text} ${units[unit]}`
}

// formatImageMeta renders an image's metadata as "1000 × 1000 · JPEG · 245 KB".
// Unknown dimensions or format (an image that failed to decode) are dropped, so
// the size alone still shows.
export function formatImageMeta(meta: ImageMeta): string {
    const parts: string[] = []
    if (meta.width > 0 && meta.height > 0) parts.push(`${meta.width} × ${meta.height}`)
    if (meta.format) parts.push(meta.format.toUpperCase())
    parts.push(formatBytes(meta.bytes))
    return parts.join(' · ')
}
