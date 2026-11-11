// A small on-device record of what the player and the page went through:
// element events, recovery attempts, play() refusals, and the page lifecycle
// (hidden, frozen, back online). Playback that stops on a locked phone leaves
// no trace anywhere else — there is no console to read on the device — so the
// About page shows this log, and it can be copied into a bug report.
//
// It survives a reload (localStorage), keeps only the latest entries, and
// never records a URL: stream URLs carry the session's apiKey.
import { readonly, ref } from 'vue'

export interface PlaybackLogEntry {
    /** Epoch milliseconds. */
    at: number
    event: string
    detail?: string
}

const STORAGE_KEY = 'aether:playbackLog'
const MAX_ENTRIES = 300

function load(): PlaybackLogEntry[] {
    try {
        const raw = localStorage.getItem(STORAGE_KEY)
        const parsed: unknown = raw ? JSON.parse(raw) : []
        return Array.isArray(parsed) ? (parsed as PlaybackLogEntry[]).slice(-MAX_ENTRIES) : []
    } catch {
        return []
    }
}

const entries = ref<PlaybackLogEntry[]>(load())

/** The entries, oldest first. */
export const playbackLog = readonly(entries)

function persist(): void {
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(entries.value))
    } catch {
        // Storage full or blocked: the in-memory log still works.
    }
}

/** Record one event. `detail` must never hold a URL or a credential. */
export function logPlayback(event: string, detail?: string): void {
    const entry: PlaybackLogEntry =
        detail === undefined ? { at: Date.now(), event } : { at: Date.now(), event, detail }
    const next = [...entries.value, entry]
    entries.value = next.length > MAX_ENTRIES ? next.slice(-MAX_ENTRIES) : next
    persist()
}

export function clearPlaybackLog(): void {
    entries.value = []
    persist()
}

/** One line per entry, local time with milliseconds — for copying into a report. */
export function formatPlaybackLog(list: readonly PlaybackLogEntry[]): string {
    return list
        .map((e) => `${formatTime(e.at)} ${e.event}${e.detail ? ` ${e.detail}` : ''}`)
        .join('\n')
}

export function formatTime(at: number): string {
    const d = new Date(at)
    const pad = (n: number, w = 2) => String(n).padStart(w, '0')
    return (
        `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ` +
        `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${pad(d.getMilliseconds(), 3)}`
    )
}

let lifecycleInstalled = false

/**
 * Record the page lifecycle next to the player's events: whether the page was
 * hidden, frozen (Chrome's Page Lifecycle `freeze`/`resume`) or offline when
 * playback stopped is exactly what the log is for. Installed once.
 */
export function installPlaybackLifecycleLog(): void {
    if (lifecycleInstalled || typeof window === 'undefined' || typeof document === 'undefined')
        return
    lifecycleInstalled = true
    document.addEventListener('visibilitychange', () =>
        logPlayback(`page-${document.visibilityState}`)
    )
    document.addEventListener('freeze', () => logPlayback('page-frozen'))
    document.addEventListener('resume', () => logPlayback('page-resumed'))
    window.addEventListener('pagehide', () => logPlayback('page-hide'))
    window.addEventListener('pageshow', (e) =>
        logPlayback('page-show', (e as PageTransitionEvent).persisted ? 'from bfcache' : undefined)
    )
    window.addEventListener('online', () => logPlayback('browser-online'))
    window.addEventListener('offline', () => logPlayback('browser-offline'))
}
