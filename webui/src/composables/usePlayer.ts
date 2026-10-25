import { ref, computed, watch } from 'vue'
import type { Song, PlayerState } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
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

// One stream-recovery attempt per loaded track: an expired apiKey kills the
// src URL, which surfaces as an element error, not a JSON response. Re-mint
// once and re-point; a second failure is a real playback error.
let streamRetriedTrackId: string | null = null

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

const recoverStream = async (el: HTMLAudioElement): Promise<void> => {
    const track = currentTrack.value
    if (!track) {
        // Nothing is playing (e.g. the queue was cleared): a media error here is
        // a deliberate stop, not a connectivity failure. Stay silent.
        return
    }
    if (streamRetriedTrackId === track.id) {
        // Already retried this track once and it failed again — a real stream
        // failure. Report it so the connectivity banner surfaces.
        const { reportNetworkError } = await import('@/composables/useConnectivity')
        reportNetworkError()
        return
    }
    streamRetriedTrackId = track.id
    const { remintApiKey } = await import('@/lib/subsonicSession')
    const result = await remintApiKey()
    if (result !== 'ok') {
        // Dead session or failed re-mint — report connectivity failure.
        const { reportNetworkError } = await import('@/composables/useConnectivity')
        reportNetworkError()
        return
    }
    const wasPlaying = isPlaying.value
    const position = el.currentTime || 0
    el.src = subsonicClient.getStreamUrl(track.id)
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
        if (wasPlaying) {
            void el.play()
        }
    }
    el.addEventListener('loadedmetadata', resume, { once: true })
    el.load()
}

const initAudio = (): void => {
    if (audio) return
    const el = new Audio()
    el.addEventListener('timeupdate', () => {
        currentTime.value = el.currentTime || 0
        maybeScrobble(el)
    })
    el.addEventListener('durationchange', () => {
        duration.value = el.duration || 0
    })
    // The next track starts within this handler, in the same task. A phone can
    // suspend a background page soon after its audio stops, so anything awaited
    // in between would wait for the user to come back to it.
    el.addEventListener('ended', () => {
        if (playNextFn) playNextFn()
    })
    el.addEventListener('play', () => {
        isPlaying.value = true
    })
    el.addEventListener('pause', () => {
        isPlaying.value = false
    })
    el.addEventListener('error', () => {
        // An error on a sourceless element is a deliberate stop (clearQueue
        // empties the src), not a stream failure — never alarm the user for it.
        if (!el.src) return
        void recoverStream(el)
    })
    audio = el
}

const getTrackUrl = (track: Song | null): string | null => {
    if (!track) return null
    if (track.streamUrl) return track.streamUrl
    if (!subsonicClient.isConfigured()) return null
    return subsonicClient.getStreamUrl(track.id)
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
        audio.play().catch((err) => {
            if (err.name !== 'AbortError') {
                console.error('Failed to play:', err)
            }
        })
    }

    const pause = (): void => {
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
        streamRetriedTrackId = null
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
        streamRetriedTrackId = null
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
