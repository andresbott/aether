import { watch } from 'vue'
import { subsonicClient } from '@/lib/api/subsonic'
import { usePlayer } from '@/composables/usePlayer'

// How often the playback position is pushed while a track plays. This is the
// resume granularity the user gets on another device: a 10-minute track picked up
// elsewhere lands within 30s of where it was left.
const POSITION_TICK_MS = 30_000

// Queue edits arrive in bursts — dragging several rows, a multi-select removal,
// an album replacing the queue — so they are coalesced instead of issuing one
// request per mutation.
const EDIT_DEBOUNCE_MS = 500

// Module-scoped so the sync is a singleton like the player itself: two mounted
// components must not run two tickers against one queue.
let ticker: ReturnType<typeof setInterval> | null = null
let editTimer: ReturnType<typeof setTimeout> | null = null
// The watch() stop handles. Held so stop() can fully unbind: PlayerLayout unmounts
// and remounts on every trip through /settings, and a start() that found stale
// watchers still "bound" would silently stop persisting for the rest of the session.
let stopWatchers: Array<() => void> = []
// Bound in start(), removed in stop(), so a beacon never fires for a player the
// layout no longer owns.
let unloadHandler: (() => void) | null = null
// The queue shape (ids + playing slot) last known to match the server, either
// because we saved it or because we just restored it. An edit whose shape equals
// this is not a change worth a request.
//
// This is what keeps restore() from echoing the server's own state back at it: a
// plain "am I restoring?" flag would not work, because the watchers restore()
// trips fire on the next tick, by which time the flag is already cleared.
let syncedSignature: string | null = null

const signatureOf = (ids: string[], index: number): string => `${index}:${ids.join(',')}`

export function useQueueSync() {
    const player = usePlayer()

    const pushQueue = (): void => {
        const ids = player.queue.value.map((s) => s.id)
        const positionMs = Math.round(player.currentTime.value * 1000)
        syncedSignature = signatureOf(ids, player.currentIndex.value)
        void subsonicClient.savePlayQueue(ids, player.currentIndex.value, positionMs)
    }

    const scheduleSave = (): void => {
        if (editTimer) clearTimeout(editTimer)
        editTimer = setTimeout(() => {
            editTimer = null
            const ids = player.queue.value.map((s) => s.id)
            if (signatureOf(ids, player.currentIndex.value) === syncedSignature) return
            pushQueue()
        }, EDIT_DEBOUNCE_MS)
    }

    const start = (): void => {
        // Idempotent: two mounted components must not run two tickers against one
        // queue. stop() clears these, so a remount rebinds cleanly.
        if (stopWatchers.length > 0) return

        // Queue content and the playing slot are the two things a restoring device
        // needs; both are saved on change. The position is NOT watched — it changes
        // every quarter second and is covered by the tick below.
        stopWatchers.push(watch(player.queue, scheduleSave, { deep: true }))
        stopWatchers.push(watch(player.currentIndex, scheduleSave))

        // Pausing is where a listener actually stops, and no tick runs while paused —
        // so without this the stored offset stays at the last tick, up to 30s behind.
        // It bypasses scheduleSave deliberately: that path skips saves whose queue
        // shape is unchanged, and on a pause only the position moved.
        stopWatchers.push(
            watch(player.isPlaying, (playing) => {
                if (playing) return
                if (player.queue.value.length === 0) return
                pushQueue()
            })
        )

        ticker = setInterval(() => {
            // A paused player is not moving, so re-saving the same offset every
            // 30s would be pure noise — and the pause itself already saved.
            if (!player.isPlaying.value) return
            if (player.queue.value.length === 0) return
            pushQueue()
        }, POSITION_TICK_MS)

        // Closing the tab cancels in-flight fetches, so the final write leaves as a
        // beacon. `pagehide` rather than `beforeunload`: it fires on mobile/back-forward
        // cache navigations too, which is exactly when a session gets abandoned.
        unloadHandler = (): void => {
            if (player.queue.value.length === 0) return
            const ids = player.queue.value.map((s) => s.id)
            const positionMs = Math.round(player.currentTime.value * 1000)
            subsonicClient.savePlayQueueBeacon(ids, player.currentIndex.value, positionMs)
        }
        window.addEventListener('pagehide', unloadHandler)
    }

    const stop = (): void => {
        stopWatchers.forEach((off) => off())
        stopWatchers = []
        if (ticker) {
            clearInterval(ticker)
            ticker = null
        }
        if (editTimer) {
            clearTimeout(editTimer)
            editTimer = null
        }
        if (unloadHandler) {
            window.removeEventListener('pagehide', unloadHandler)
            unloadHandler = null
        }
    }

    // Adopts the queue saved by another browser or device. Deliberately restores
    // PAUSED: browsers block autoplay, and resuming audio unprompted on page load
    // is hostile.
    //
    // The saved position belongs to the saved current track alone, so it is
    // applied via the player's resume path — stepping to any other track in the
    // restored queue starts from the beginning, as it should.
    const restore = async (): Promise<void> => {
        const saved = await subsonicClient.getPlayQueue()
        if (!saved || saved.entry.length === 0) return

        const index =
            saved.currentIndex < 0 ? 0 : Math.min(saved.currentIndex, saved.entry.length - 1)
        // The offset is only meaningful for the track the server says was current;
        // a clamped index means that track is not the one we landed on, so start
        // from the top.
        const seconds =
            saved.currentIndex === index && saved.currentIndex >= 0
                ? Math.max(0, saved.position / 1000)
                : 0
        // Mark the restored shape as already in sync, so the watchers this trips do
        // not echo it back — a save would rewrite changedBy and clobber the very
        // state we just adopted.
        syncedSignature = signatureOf(
            saved.entry.map((s) => s.id),
            index
        )
        player.restoreSession(saved.entry, index, seconds)
    }

    return { start, stop, restore }
}
