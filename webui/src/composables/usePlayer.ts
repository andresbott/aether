import { ref, computed, watch } from 'vue'
import type { Song, PlayerState } from '@/types/subsonic'
import { subsonicClient, type SessionCheck } from '@/lib/api/subsonic'
import { httpFetch } from '@/lib/host'
import { installPlaybackLifecycleLog, logPlayback } from '@/lib/playbackLog'
import { onReconnect, reportNetworkError } from '@/composables/useConnectivity'
import { saveToLocalStorage, loadFromLocalStorage } from '@/utils/localStorage'
import { reorderQueue } from '@/utils/queueReorder'
import {
    nextQueueIndex,
    buildShuffleOrder,
    resyncShuffleOrder,
    stepOrderPosition
} from '@/utils/playbackOrder'

const STORAGE_KEY_QUEUE = 'musicPlayer:queue'
const STORAGE_KEY_CURRENT_INDEX = 'musicPlayer:currentIndex'
const STORAGE_KEY_VOLUME = 'musicPlayer:volume'
const STORAGE_KEY_UNMUTED_VOLUME = 'musicPlayer:unmutedVolume'
const STORAGE_KEY_REPEAT = 'musicPlayer:repeat'
const STORAGE_KEY_SHUFFLE = 'musicPlayer:shuffle'
const STORAGE_KEY_SHUFFLE_ORDER = 'musicPlayer:shuffleOrder'

const currentTrack = ref<Song | null>(null)
const queue = ref<Song[]>(loadFromLocalStorage<Song[]>(STORAGE_KEY_QUEUE, []))
const currentIndex = ref<number>(loadFromLocalStorage<number>(STORAGE_KEY_CURRENT_INDEX, 0))
const isPlaying = ref<boolean>(false)
const volume = ref<number>(loadFromLocalStorage<number>(STORAGE_KEY_VOLUME, 1))
// The volume the speaker button comes back to. Every non-zero volume is recorded
// here, so silence reached by dragging the rail to 0 unmutes just like silence
// reached by clicking the speaker. Persisted separately from `volume` so a
// session that was left muted still knows where to return.
const unmutedVolume = ref<number>(loadFromLocalStorage<number>(STORAGE_KEY_UNMUTED_VOLUME, 1))
const repeat = ref<'none' | 'all'>(loadFromLocalStorage<'none' | 'all'>(STORAGE_KEY_REPEAT, 'none'))
const shuffle = ref<boolean>(loadFromLocalStorage<boolean>(STORAGE_KEY_SHUFFLE, false))
const currentTime = ref<number>(0)
const duration = ref<number>(0)
// A random play order over the queue, held as track ids alongside the queue's
// own order. With shuffle on, next/previous walk this permutation instead of the
// queue, so the random sequence is stable and can be stepped backwards. Rebuilt
// when shuffle is switched on or the queue is replaced.
const shuffleOrder = ref<string[]>(loadFromLocalStorage<string[]>(STORAGE_KEY_SHUFFLE_ORDER, []))

// One audio element plays every track: a track change re-points its src. On a
// phone that is what keeps an album going — mobile Safari lets an element start
// sound on its own only once a tap has started it, so every later track has to
// play on the element the user's tap unlocked: from the `ended` handler, with
// nobody tapping and maybe the screen locked. (A second element pre-buffering
// the next track was tried and removed; see docs/agents/frontend.md.)
let audio: HTMLAudioElement | null = null
let playNextFn: (() => void) | null = null

// Scrobble bookkeeping. Last.fm's rule: a play counts once it passes half the
// track or four minutes, whichever comes first. Tracks under 30s never count.
// `scrobbledTrackId` makes it once-per-loaded-track, so seeking back and forth
// across the threshold cannot double-submit.
const SCROBBLE_MIN_DURATION = 30
const SCROBBLE_CAP_SECONDS = 240
let scrobbledTrackId: string | null = null

// Whether the user wants sound: set by play(), cleared by a pause — theirs, or
// one the system made (headphones unplugged, a call) — by clearing the queue,
// and by the queue running out. Recovery and the stall watchdog act only while
// it is set, so they never start audio nobody asked for. Distinct from
// isPlaying, which follows the element and is false while a track loads.
let wantsPlayback = false

const maybeScrobble = (el: HTMLAudioElement): void => {
    const track = currentTrack.value
    if (!track || scrobbledTrackId === track.id) return
    const total = el.duration || track.duration || 0
    if (total < SCROBBLE_MIN_DURATION) return
    const threshold = Math.min(total / 2, SCROBBLE_CAP_SECONDS)
    if ((el.currentTime || 0) < threshold) return
    scrobbledTrackId = track.id
    void subsonicClient.scrobble(track.id)
}

const getTrackUrl = (track: Song | null): string | null => {
    if (!track) return null
    if (track.streamUrl) return track.streamUrl
    if (!subsonicClient.isConfigured()) return null
    return subsonicClient.getStreamUrl(track.id)
}

// ---- Stream recovery --------------------------------------------------------
//
// A track that fails to load — an element error, a play() the browser rejected
// for a reason other than autoplay, or a load the stall watchdog gave up on — is
// recovered in place, at its position. A phone that slept for minutes is the
// usual cause: the next track is the first request in a while, and it meets a
// dozing radio, a Wi-Fi ↔ mobile handover or an expired key. Each attempt first
// asks the server with a cheap ping (subsonicClient.checkSession), which also
// re-mints an expired key, and only then re-points the element:
// - the server answers → re-point at the fresh URL and resume;
// - no answer → raise the connectivity banner and try again after a backoff,
//   or as soon as the connectivity probe sees the server again (onReconnect);
// - the session is refused → stop; the session handling owns that.
// A track that keeps failing while the server answers is unplayable, and is
// skipped rather than retried forever.

// Waits before the second and later attempts; the first runs at once.
const RECOVERY_DELAYS_MS = [1_000, 2_000, 4_000, 8_000, 15_000, 30_000]
// Re-points that failed again although the server answered.
const MAX_FAILURES_WHILE_REACHABLE = 2

interface Recovery {
    trackId: string
    reason: string
    attempts: number
    failuresWhileReachable: number
    // Where to resume: the position the failure interrupted, kept across
    // attempts (a fresh src reads 0 until its metadata loads).
    position: number
    timer: ReturnType<typeof setTimeout> | null
    running: boolean
    // The last check found no server: a reconnect is worth acting on.
    waitingForNetwork: boolean
}

let recovery: Recovery | null = null

const cancelRecovery = (): void => {
    if (recovery?.timer) clearTimeout(recovery.timer)
    recovery = null
}

const recover = (el: HTMLAudioElement, reason: string): void => {
    const track = currentTrack.value
    if (!track) {
        // Nothing is playing (e.g. the queue was cleared): a media error here is
        // a deliberate stop, not a connectivity failure. Stay silent.
        return
    }
    const position = el.currentTime || 0
    if (!recovery || recovery.trackId !== track.id) {
        cancelRecovery()
        recovery = {
            trackId: track.id,
            reason,
            attempts: 0,
            failuresWhileReachable: 0,
            position,
            timer: null,
            running: false,
            waitingForNetwork: false
        }
    } else {
        recovery.reason = reason
        if (position > 0) recovery.position = position
    }
    // One attempt at a time: an error and a rejected play() for the same load
    // arrive together.
    if (recovery.running || recovery.timer) return
    if (recovery.attempts === 0) {
        void attemptRecovery(el)
        return
    }
    scheduleRecovery(el)
}

const scheduleRecovery = (el: HTMLAudioElement): void => {
    const r = recovery
    if (!r || r.timer || r.running) return
    const index = r.attempts - 1
    if (index >= RECOVERY_DELAYS_MS.length) {
        // Out of timed attempts. A reconnect can still bring the track back
        // while the user wants it (see onReconnect below).
        logPlayback('recover-gave-up', `${r.trackId} after ${r.attempts} attempts`)
        return
    }
    r.timer = setTimeout(() => {
        r.timer = null
        if (recovery === r) void attemptRecovery(el)
    }, RECOVERY_DELAYS_MS[index])
}

// checkSession turns every failure into an answer; this keeps an unexpected
// throw from leaving a recovery marked as running forever.
const checkSession = async (): Promise<SessionCheck> => {
    try {
        return await subsonicClient.checkSession()
    } catch {
        return 'unreachable'
    }
}

const attemptRecovery = async (el: HTMLAudioElement): Promise<void> => {
    const r = recovery
    if (!r) return
    r.running = true
    r.attempts++
    logPlayback('recover', `${r.trackId} attempt ${r.attempts} (${r.reason})`)
    const track = currentTrack.value
    // A direct stream URL (a radio station) is not this server's: re-point it
    // without asking.
    const status = track?.streamUrl ? 'ok' : await checkSession()
    r.running = false
    // A track change or a cleared queue overtook the check.
    if (recovery !== r || currentTrack.value?.id !== r.trackId) return
    logPlayback('session-check', status)
    if (status === 'unreachable') {
        r.waitingForNetwork = true
        reportNetworkError()
        scheduleRecovery(el)
        return
    }
    r.waitingForNetwork = false
    if (status === 'refused') {
        // A dead session: in a cookie session the re-mint already failed with a
        // 401 and the login gate is coming; a refused key opens the key card.
        // Nothing to retry until then, so the watchdog stands down too.
        cancelRecovery()
        wantsPlayback = false
        disarmStallWatchdog()
        return
    }
    // Only a decode or format error says the track itself is bad; a broken-off
    // transfer or a stall on a slow link is the network's, and is retried.
    if (r.attempts > 1 && r.reason === 'error') r.failuresWhileReachable++
    if (r.failuresWhileReachable >= MAX_FAILURES_WHILE_REACHABLE) {
        logPlayback('skip-unplayable', r.trackId)
        cancelRecovery()
        if (wantsPlayback && playNextFn) playNextFn()
        return
    }
    repoint(el, r.position)
}

// Re-point the element at the current track's URL (with whatever key is
// installed now) and resume at `position`.
const repoint = (el: HTMLAudioElement, position: number): void => {
    const url = getTrackUrl(currentTrack.value)
    if (!url) return
    el.src = url
    el.volume = volume.value
    // A fresh src resets the element to "no metadata", where assigning
    // currentTime is dropped (or throws InvalidStateError). Restore the position
    // once the new stream is seekable, then pick playback back up.
    const resume = (): void => {
        if (position > 0) {
            try {
                el.currentTime = position
            } catch {
                // Non-seekable stream: resume from the start rather than fail.
            }
        }
        if (wantsPlayback) startPlayback(el)
    }
    el.addEventListener('loadedmetadata', resume, { once: true })
    el.load()
    // A re-point that never gets as far as metadata is a stall too.
    if (wantsPlayback) armStallWatchdog(el, 'reload')
}

// ---- Stall watchdog ---------------------------------------------------------
//
// A load the network swallows does not always end in an error: the element can
// sit in 'waiting' forever. While the user wants sound, a position that has not
// moved for STALL_TIMEOUT_MS after the element said it was waiting (or after a
// play() call) is treated as a failed load and recovered.
const STALL_TIMEOUT_MS = 15_000
let stallTimer: ReturnType<typeof setTimeout> | null = null
let stallMark = 0

const disarmStallWatchdog = (): void => {
    if (stallTimer) clearTimeout(stallTimer)
    stallTimer = null
}

const armStallWatchdog = (el: HTMLAudioElement, why: string): void => {
    if (!wantsPlayback || !currentTrack.value || stallTimer) return
    stallMark = el.currentTime || 0
    stallTimer = setTimeout(() => {
        stallTimer = null
        if (!wantsPlayback || !currentTrack.value) return
        if (!el.paused && (el.currentTime || 0) > stallMark + 0.5) return
        logPlayback('stall-timeout', `${currentTrack.value.id} (${why})`)
        recover(el, 'stall')
    }, STALL_TIMEOUT_MS)
}

// ---- Starting playback ------------------------------------------------------

const startPlayback = (el: HTMLAudioElement): void => {
    wantsPlayback = true
    armStallWatchdog(el, 'start')
    el.play().catch((err: { name?: string; message?: string }) => {
        // A newer src or pause() interrupted this play() — not a failure.
        if (err?.name === 'AbortError') return
        logPlayback('play-rejected', err?.name ?? 'unknown')
        if (err?.name === 'NotAllowedError') {
            // Autoplay policy: nothing here can start sound until the user taps
            // play. Stop wanting it, so the watchdog does not retry in a loop.
            wantsPlayback = false
            disarmStallWatchdog()
            return
        }
        console.error('Failed to play:', err)
        recover(el, 'play-rejected')
    })
}

// ---- Warming the next track -------------------------------------------------
//
// Near the end of a track, ask the server for the first bytes of the next one.
// It wakes the radio and the connection while the page is still busy playing,
// and finds a dead key or a dead network before the handoff needs them: the
// check re-mints an expired key, so the next track's URL is built with a good
// one. The answer is thrown away; the element fetches the track itself.
const WARM_AHEAD_SECONDS = 30
const WARM_RANGE = 'bytes=0-65535'
let warmedForTrackId: string | null = null

const maybeWarmNext = (el: HTMLAudioElement): void => {
    const track = currentTrack.value
    if (!wantsPlayback || !track || warmedForTrackId === track.id || track.streamUrl) return
    const total = el.duration || track.duration || 0
    if (!total || !isFinite(total) || total - (el.currentTime || 0) > WARM_AHEAD_SECONDS) return
    warmedForTrackId = track.id
    const nextIndex = resolveStep(1)
    const next = nextIndex === null ? null : queue.value[nextIndex]
    if (!next || next.streamUrl || !subsonicClient.isConfigured()) return
    void warmUp(next.id)
}

const warmUp = async (id: string): Promise<void> => {
    try {
        const response = await httpFetch(subsonicClient.getStreamUrl(id), {
            headers: { Range: WARM_RANGE }
        })
        const type = response.headers.get('content-type') ?? ''
        // Drain the body so the connection can be reused.
        await response.arrayBuffer().catch(() => undefined)
        // A refused key answers a Subsonic error document, not audio.
        if (response.ok && !type.includes('json') && !type.includes('xml')) {
            logPlayback('warm-next', `${id} ok`)
            return
        }
        logPlayback('warm-next', `${id} refused (${response.status})`)
    } catch {
        logPlayback('warm-next', `${id} no answer`)
    }
    const status = await checkSession()
    logPlayback('session-check', status)
    if (status === 'unreachable') reportNetworkError()
}

const initAudio = (): void => {
    if (audio) return
    installPlaybackLifecycleLog()
    const el = new Audio()
    el.addEventListener('timeupdate', () => {
        const now = el.currentTime || 0
        currentTime.value = now
        maybeScrobble(el)
        if (stallTimer && now > stallMark + 0.5) disarmStallWatchdog()
        // Well past where it was recovered: the track is healthy again.
        if (
            recovery &&
            recovery.trackId === currentTrack.value?.id &&
            now > recovery.position + 5
        ) {
            cancelRecovery()
        }
        maybeWarmNext(el)
    })
    el.addEventListener('durationchange', () => {
        duration.value = el.duration || 0
    })
    // The next track starts within this handler, in the same task. A phone can
    // suspend a background page soon after its audio stops, so anything awaited
    // in between would wait for the user to come back to it.
    el.addEventListener('ended', () => {
        logPlayback('ended', currentTrack.value?.id)
        if (playNextFn) playNextFn()
    })
    el.addEventListener('play', () => {
        isPlaying.value = true
    })
    el.addEventListener('playing', () => {
        logPlayback('playing', currentTrack.value?.id)
    })
    el.addEventListener('pause', () => {
        isPlaying.value = false
        // The pause a track's end brings is not the user's: the next track is
        // about to start. Any other pause — the user's, or one the system made
        // (headphones unplugged, a call) — means they no longer want sound. A
        // pause event that arrives after play() was called again (queued by a
        // src change, say) is stale: the element is not paused any more.
        if (el.ended || !el.paused) return
        wantsPlayback = false
        disarmStallWatchdog()
        logPlayback('paused', currentTrack.value?.id)
    })
    const onWaiting = (event: Event): void => {
        logPlayback(event.type, currentTrack.value?.id)
        armStallWatchdog(el, event.type)
    }
    el.addEventListener('waiting', onWaiting)
    el.addEventListener('stalled', onWaiting)
    el.addEventListener('error', () => {
        // An error on a sourceless element is a deliberate stop (clearQueue
        // empties the src), not a stream failure — never alarm the user for it.
        if (!el.src || !currentTrack.value) return
        const code = el.error?.code
        logPlayback('error', `${currentTrack.value.id} code ${code ?? '?'}`)
        // MEDIA_ERR_NETWORK: the transfer broke off, the file is fine.
        recover(el, code === 2 ? 'network' : 'error')
    })
    // The server is back after an outage: a track that died with the network,
    // or is still waiting out a backoff, gets a fresh attempt now.
    onReconnect(() => {
        if (!wantsPlayback || !currentTrack.value) return
        const r = recovery
        if (!r || r.trackId !== currentTrack.value.id || r.running) return
        if (!r.waitingForNetwork) return
        logPlayback('reconnected', r.trackId)
        if (r.timer) clearTimeout(r.timer)
        r.timer = null
        void attemptRecovery(el)
    })
    audio = el
}

// Draw a fresh random order over the queue, starting at the given track so the
// shuffled run begins where playback currently is.
const rebuildShuffleOrder = (startId?: string): void => {
    const ids = queue.value.map((s) => s.id)
    const first = startId ?? queue.value[currentIndex.value]?.id ?? null
    shuffleOrder.value = buildShuffleOrder(ids, first)
}

// Fold queue changes into the existing random order: removed tracks drop out,
// added ones get random slots among the entries that have not played yet.
// Keeping the order rather than redrawing it means a queue edit never
// re-randomizes the run the user is already listening to.
const syncShuffleOrder = (): void => {
    if (!shuffle.value && shuffleOrder.value.length === 0) return
    shuffleOrder.value = resyncShuffleOrder(
        shuffleOrder.value,
        queue.value.map((s) => s.id),
        queue.value[currentIndex.value]?.id ?? null
    )
}

// Where the playing track sits in the random order, or -1 when it is not in it
// (a freshly inserted track, or an order that has not been built yet).
const shufflePosition = (): number => {
    const id = queue.value[currentIndex.value]?.id
    return id === undefined ? -1 : shuffleOrder.value.indexOf(id)
}

// The queue index one step away from the current one: the adjacent queue slot
// with shuffle off, the adjacent entry of the random order with shuffle on.
// Returns null when there is nothing in that direction.
const resolveStep = (delta: 1 | -1): number | null => {
    if (!shuffle.value) {
        if (delta === 1) return nextQueueIndex(currentIndex.value, queue.value.length, repeat.value)
        const prev = currentIndex.value - 1
        if (prev >= 0) return prev
        return repeat.value === 'all' && queue.value.length > 0 ? queue.value.length - 1 : null
    }

    const position = stepOrderPosition(
        shufflePosition(),
        shuffleOrder.value.length,
        delta,
        repeat.value
    )
    if (position === null) return null
    const id = shuffleOrder.value[position]
    if (id === undefined) return null
    const index = queue.value.findIndex((s) => s.id === id)
    return index === -1 ? null : index
}

export function usePlayer() {
    initAudio()

    const playerState = computed<PlayerState>(() => ({
        currentTrack: currentTrack.value,
        queue: queue.value,
        currentIndex: currentIndex.value,
        isPlaying: isPlaying.value,
        volume: volume.value,
        repeat: repeat.value,
        shuffle: shuffle.value,
        currentTime: currentTime.value,
        duration: duration.value
    }))

    if (queue.value.length > 0 && currentIndex.value < queue.value.length) {
        currentTrack.value = queue.value[currentIndex.value] || null
    }

    if (audio && !audio.src) {
        const url = getTrackUrl(currentTrack.value)
        if (url) {
            audio.src = url
            audio.volume = volume.value
        }
    }
    // A restored session may have shuffle on with an order that predates the
    // stored queue, or none at all.
    if (shuffle.value) {
        if (shuffleOrder.value.length === 0) rebuildShuffleOrder()
        else syncShuffleOrder()
    }

    watch(
        queue,
        (newQueue) => {
            saveToLocalStorage(STORAGE_KEY_QUEUE, newQueue)
        },
        { deep: true }
    )

    watch(currentIndex, (newIndex) => {
        saveToLocalStorage(STORAGE_KEY_CURRENT_INDEX, newIndex)
    })

    watch(repeat, (newRepeat) => {
        saveToLocalStorage(STORAGE_KEY_REPEAT, newRepeat)
    })

    watch(shuffle, (newShuffle) => {
        saveToLocalStorage(STORAGE_KEY_SHUFFLE, newShuffle)
    })

    watch(shuffleOrder, (newOrder) => {
        saveToLocalStorage(STORAGE_KEY_SHUFFLE_ORDER, newOrder)
    })

    const hasNext = computed(() => {
        if (queue.value.length === 0) return false
        if (repeat.value === 'all') return true
        if (shuffle.value) {
            // At the end of the random order there is nothing left to play.
            const position = shufflePosition()
            return position < shuffleOrder.value.length - 1
        }
        return currentIndex.value < queue.value.length - 1
    })

    const hasPrevious = computed(() => {
        if (queue.value.length === 0) return false
        if (repeat.value === 'all') return true
        if (shuffle.value) return shufflePosition() !== 0
        return currentIndex.value > 0
    })

    const isMuted = computed(() => volume.value === 0)

    // Sync flush: the audible level should follow the rail without waiting for a
    // tick, and `unmutedVolume` has to be current the instant the volume lands so
    // a mute in the same tick still knows where to come back to.
    watch(
        volume,
        (newVolume) => {
            if (audio) audio.volume = newVolume
            saveToLocalStorage(STORAGE_KEY_VOLUME, newVolume)
            if (newVolume > 0) {
                unmutedVolume.value = newVolume
                saveToLocalStorage(STORAGE_KEY_UNMUTED_VOLUME, newVolume)
            }
        },
        { flush: 'sync' }
    )

    const play = (): void => {
        if (!currentTrack.value && queue.value.length > 0) {
            loadTrack(0)
        }
        if (!audio || !audio.src) return
        startPlayback(audio)
    }

    const pause = (): void => {
        wantsPlayback = false
        disarmStallWatchdog()
        audio?.pause()
        isPlaying.value = false
    }

    const togglePlayPause = (): void => {
        if (isPlaying.value) {
            pause()
        } else {
            play()
        }
    }

    const loadTrack = (index: number): void => {
        if (index < 0 || index >= queue.value.length) return
        currentIndex.value = index
        currentTrack.value = queue.value[index] || null
        currentTime.value = 0
        // The catalog's length until the element reports the stream's own, so
        // the timeline — and a phone's lock screen — reads right while it loads.
        duration.value = currentTrack.value?.duration ?? 0
        scrobbledTrackId = null
        // A new load: whatever was being recovered or watched belonged to the
        // track before.
        cancelRecovery()
        disarmStallWatchdog()
        logPlayback('load', currentTrack.value?.id)
        const url = getTrackUrl(currentTrack.value)
        if (url && audio) {
            audio.src = url
            audio.volume = volume.value
        }
    }

    const playNext = (): void => {
        const nextIndex = resolveStep(1)
        if (nextIndex === null) {
            pause()
            return
        }
        loadTrack(nextIndex)
        play()
    }

    const playPrevious = (): void => {
        if (currentTime.value > 3) {
            if (audio) audio.currentTime = 0
            return
        }

        const prevIndex = resolveStep(-1)
        if (prevIndex === null) {
            if (audio) audio.currentTime = 0
            return
        }

        loadTrack(prevIndex)
        play()
    }

    const seek = (time: number): void => {
        if (audio) audio.currentTime = time
        currentTime.value = time
    }

    const setVolume = (newVolume: number): void => {
        volume.value = Math.max(0, Math.min(1, newVolume))
    }

    const toggleMute = (): void => {
        // Restore to full volume when the remembered level is itself silent —
        // clicking the speaker must always produce sound.
        setVolume(isMuted.value ? unmutedVolume.value || 1 : 0)
    }

    const toggleRepeat = (): void => {
        repeat.value = repeat.value === 'all' ? 'none' : 'all'
    }

    const toggleShuffle = (): void => {
        shuffle.value = !shuffle.value
        // Turning shuffle on draws a new random order from the playing track;
        // turning it off drops it so the next enable is not a stale sequence.
        if (shuffle.value) rebuildShuffleOrder()
        else shuffleOrder.value = []
    }

    const addToQueue = (song: Song): void => {
        queue.value.push(song)
        syncShuffleOrder()
    }

    const addMultipleToQueue = (songs: Song[]): void => {
        queue.value.push(...songs)
        syncShuffleOrder()
    }

    // The track-list double-click gesture: the songs land at the END of the queue
    // instead of replacing it, so a double-click never discards what is queued.
    // When nothing is loaded the queue was idle and there would be no audible
    // result, so the first appended track starts playing.
    const enqueueAndPlayIfIdle = (songs: Song[]): void => {
        if (songs.length === 0) return
        const wasIdle = currentTrack.value === null || queue.value.length === 0
        const startIndex = queue.value.length
        queue.value.push(...songs)
        if (wasIdle) {
            // Nothing was playing, so there is no run to preserve: draw the random
            // order around the track that is about to start.
            if (shuffle.value) rebuildShuffleOrder(songs[0]?.id)
            loadTrack(startIndex)
            play()
            return
        }
        syncShuffleOrder()
    }

    const playNow = (song: Song): void => {
        queue.value = [song]
        shuffleOrder.value = shuffle.value ? [song.id] : []
        loadTrack(0)
        play()
    }

    const playAlbum = (songs: Song[], startIndex: number = 0): void => {
        queue.value = [...songs]
        // A new queue invalidates the old order entirely: redraw it around the
        // track the user picked.
        if (shuffle.value) rebuildShuffleOrder(songs[startIndex]?.id)
        else shuffleOrder.value = []
        loadTrack(startIndex)
        play()
    }

    const removeFromQueue = (index: number): void => {
        if (index === currentIndex.value) {
            playNext()
        } else if (index < currentIndex.value) {
            currentIndex.value--
        }
        queue.value.splice(index, 1)
        syncShuffleOrder()
    }

    const removeManyFromQueue = (indices: number[]): void => {
        if (indices.length === 0) return
        const toRemove = new Set(indices)
        const current = queue.value[currentIndex.value] ?? null
        const currentRemoved = toRemove.has(currentIndex.value)
        // How many kept tracks precede the current one — the slot the next
        // surviving track falls into when the current track itself is removed.
        const keptBeforeCurrent = queue.value
            .slice(0, currentIndex.value)
            .reduce((n, _, i) => (toRemove.has(i) ? n : n + 1), 0)
        const next = queue.value.filter((_, i) => !toRemove.has(i))

        if (next.length === 0) {
            clearQueue()
            return
        }

        queue.value = next
        if (currentRemoved) {
            // The playing track is gone: advance to whatever fell into its slot
            // (or the new last track) and keep playback going if it was.
            const wasPlaying = isPlaying.value
            loadTrack(Math.min(keptBeforeCurrent, next.length - 1))
            syncShuffleOrder()
            if (wasPlaying) play()
        } else if (current) {
            // Keep pointing at the still-playing track.
            const idx = next.indexOf(current)
            if (idx !== -1) currentIndex.value = idx
            syncShuffleOrder()
        }
    }

    const moveInQueue = (fromIndices: number[], targetIndex: number): void => {
        if (fromIndices.length === 0) return
        const current = queue.value[currentIndex.value] ?? null
        const next = reorderQueue(queue.value, fromIndices, targetIndex)
        queue.value = next
        if (current) {
            const idx = next.indexOf(current)
            if (idx !== -1) currentIndex.value = idx
        }
    }

    const insertIntoQueue = (songs: Song[], targetIndex: number): void => {
        if (songs.length === 0) return
        const clamped = Math.max(0, Math.min(targetIndex, queue.value.length))
        const current = queue.value[currentIndex.value] ?? null
        queue.value = [...queue.value.slice(0, clamped), ...songs, ...queue.value.slice(clamped)]
        if (current) {
            const idx = queue.value.indexOf(current)
            if (idx !== -1) currentIndex.value = idx
        }
        syncShuffleOrder()
    }

    const clearQueue = (): void => {
        queue.value = []
        shuffleOrder.value = []
        currentIndex.value = 0
        currentTrack.value = null
        // Per-track bookkeeping belongs to a track that no longer exists; a
        // logout purge clears the queue, and the next session's first track must
        // start with a fresh scrobble and stream-recovery budget.
        scrobbledTrackId = null
        cancelRecovery()
        pause()
        if (audio) audio.src = ''
    }

    // Picking a track by hand jumps to its slot in the random order rather than
    // redrawing, so next/previous keep walking the same sequence around it.
    const playQueueItem = (index: number): void => {
        loadTrack(index)
        play()
    }

    // Adopts a queue saved elsewhere (see useQueueSync), positioning the current
    // track at `seconds` without starting playback — browsers block autoplay, and
    // resuming audio unprompted on load is hostile.
    //
    // The offset is applied here rather than via seek() so it survives loadTrack's
    // reset: `seconds` belongs to THIS track only, and stepping to any other slot
    // goes through the normal loadTrack path and starts at 0.
    const restoreSession = (songs: Song[], index: number, seconds: number): void => {
        if (songs.length === 0) return
        queue.value = [...songs]
        shuffleOrder.value = []
        const clamped = Math.max(0, Math.min(index, songs.length - 1))
        loadTrack(clamped)
        if (shuffle.value) rebuildShuffleOrder(songs[clamped]?.id)
        if (seconds > 0) {
            currentTime.value = seconds
            // The element may not have metadata yet, so the assignment can be
            // dropped by the browser; the timeline still reads correct and the
            // pending seek is re-applied once the track is loadable.
            if (audio) {
                const el = audio
                const applySeek = (): void => {
                    el.currentTime = seconds
                }
                applySeek()
                el.addEventListener('loadedmetadata', applySeek, { once: true })
            }
        }
    }

    playNextFn = playNext

    return {
        playerState,
        currentTrack,
        queue,
        currentIndex,
        isPlaying,
        volume,
        isMuted,
        repeat,
        shuffle,
        currentTime,
        duration,
        shuffleOrder,
        hasNext,
        hasPrevious,
        play,
        pause,
        togglePlayPause,
        playNext,
        playPrevious,
        seek,
        setVolume,
        toggleMute,
        toggleRepeat,
        toggleShuffle,
        addToQueue,
        addMultipleToQueue,
        enqueueAndPlayIfIdle,
        playNow,
        playAlbum,
        removeFromQueue,
        removeManyFromQueue,
        moveInQueue,
        insertIntoQueue,
        clearQueue,
        playQueueItem,
        restoreSession
    }
}
