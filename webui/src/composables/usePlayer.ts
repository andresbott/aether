import { ref, computed, watch } from 'vue'
import type { Song, PlayerState } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { saveToLocalStorage, loadFromLocalStorage } from '@/utils/localStorage'
import { reorderQueue } from '@/utils/queueReorder'
import { nextQueueIndex } from '@/utils/playbackOrder'

const STORAGE_KEY_QUEUE = 'musicPlayer:queue'
const STORAGE_KEY_CURRENT_INDEX = 'musicPlayer:currentIndex'
const STORAGE_KEY_VOLUME = 'musicPlayer:volume'
const STORAGE_KEY_REPEAT = 'musicPlayer:repeat'
const STORAGE_KEY_SHUFFLE = 'musicPlayer:shuffle'

const currentTrack = ref<Song | null>(null)
const queue = ref<Song[]>(loadFromLocalStorage<Song[]>(STORAGE_KEY_QUEUE, []))
const currentIndex = ref<number>(loadFromLocalStorage<number>(STORAGE_KEY_CURRENT_INDEX, 0))
const isPlaying = ref<boolean>(false)
const volume = ref<number>(loadFromLocalStorage<number>(STORAGE_KEY_VOLUME, 1))
const repeat = ref<'none' | 'all' | 'one'>(
    loadFromLocalStorage<'none' | 'all' | 'one'>(STORAGE_KEY_REPEAT, 'none')
)
const shuffle = ref<boolean>(loadFromLocalStorage<boolean>(STORAGE_KEY_SHUFFLE, false))
const currentTime = ref<number>(0)
const duration = ref<number>(0)
// The track buffered ahead on the standby element, so the next track can start
// without a network/decode gap. null when nothing is queued to play next.
const preloadedTrack = ref<Song | null>(null)

// Two audio elements are kept alive at once: `activeEl` plays the current track
// while `standbyEl` pre-buffers the next one. On track end we swap their roles
// instead of re-pointing a single element's src, which removes the gap that a
// fresh network request + decode would otherwise introduce.
let activeEl: HTMLAudioElement | null = null
let standbyEl: HTMLAudioElement | null = null
let preloadedUrl: string | null = null
let playNextFn: (() => void) | null = null

const attachListeners = (el: HTMLAudioElement): void => {
    el.addEventListener('timeupdate', () => {
        if (el !== activeEl) return
        currentTime.value = el.currentTime || 0
    })
    el.addEventListener('durationchange', () => {
        if (el !== activeEl) return
        duration.value = el.duration || 0
    })
    el.addEventListener('ended', () => {
        if (el !== activeEl) return
        if (playNextFn) playNextFn()
    })
    el.addEventListener('play', () => {
        if (el !== activeEl) return
        isPlaying.value = true
    })
    el.addEventListener('pause', () => {
        if (el !== activeEl) return
        isPlaying.value = false
    })
}

const initAudioElements = (): void => {
    if (activeEl) return
    activeEl = new Audio()
    standbyEl = new Audio()
    attachListeners(activeEl)
    attachListeners(standbyEl)
}

const getTrackUrl = (track: Song | null): string | null => {
    if (!track) return null
    if (track.streamUrl) return track.streamUrl
    if (!subsonicClient.isConfigured()) return null
    return subsonicClient.getStreamUrl(track.id)
}

// Point the standby element at whatever track should play next so the browser
// can buffer it ahead of time. Repeat 'one' replays the current track, so there
// is nothing distinct to pre-buffer.
const updatePreload = (): void => {
    if (!standbyEl) return
    const nextIndex =
        repeat.value === 'one'
            ? null
            : nextQueueIndex(currentIndex.value, queue.value.length, repeat.value)
    const nextTrack = nextIndex === null ? null : (queue.value[nextIndex] ?? null)
    preloadedTrack.value = nextTrack

    const url = getTrackUrl(nextTrack)
    if (url === preloadedUrl) return
    preloadedUrl = url
    if (url) {
        standbyEl.src = url
        standbyEl.preload = 'auto'
        standbyEl.volume = volume.value
        standbyEl.load()
    } else {
        standbyEl.src = ''
    }
}

const swapToStandby = (): void => {
    const previousActive = activeEl
    activeEl = standbyEl
    standbyEl = previousActive
    // The element that is now standby no longer holds the upcoming track, so
    // force the next updatePreload() to re-point it.
    preloadedUrl = null
}

export function usePlayer() {
    initAudioElements()

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

    if (activeEl && !activeEl.src) {
        const url = getTrackUrl(currentTrack.value)
        if (url) {
            activeEl.src = url
            activeEl.volume = volume.value
        }
    }
    updatePreload()

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
        updatePreload()
    })

    watch(shuffle, (newShuffle) => {
        saveToLocalStorage(STORAGE_KEY_SHUFFLE, newShuffle)
    })

    const hasNext = computed(() => {
        if (repeat.value === 'one') return true
        if (repeat.value === 'all') return queue.value.length > 0
        return currentIndex.value < queue.value.length - 1
    })

    const hasPrevious = computed(() => {
        if (repeat.value === 'all') return queue.value.length > 0
        return currentIndex.value > 0
    })

    watch(volume, (newVolume) => {
        if (activeEl) activeEl.volume = newVolume
        if (standbyEl) standbyEl.volume = newVolume
        saveToLocalStorage(STORAGE_KEY_VOLUME, newVolume)
    })

    const play = (): void => {
        if (!currentTrack.value && queue.value.length > 0) {
            loadTrack(0)
        }
        if (!activeEl || !activeEl.src) return
        activeEl.play().catch((err) => {
            if (err.name !== 'AbortError') {
                console.error('Failed to play:', err)
            }
        })
    }

    const pause = (): void => {
        activeEl?.pause()
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
        const url = getTrackUrl(currentTrack.value)
        if (url && activeEl) {
            activeEl.src = url
            activeEl.volume = volume.value
        }
        updatePreload()
    }

    const playNext = (): void => {
        if (repeat.value === 'one') {
            if (currentTrack.value && activeEl) {
                activeEl.currentTime = 0
                play()
            }
            return
        }

        const nextIndex = nextQueueIndex(currentIndex.value, queue.value.length, repeat.value)
        if (nextIndex === null) {
            pause()
            return
        }

        // Fast path: the standby element already buffered this exact track, so
        // swap to it instead of issuing a fresh request.
        if (standbyEl && preloadedTrack.value && preloadedTrack.value === queue.value[nextIndex]) {
            swapToStandby()
            currentIndex.value = nextIndex
            currentTrack.value = queue.value[nextIndex] || null
            // The swapped-in element already buffered its metadata while on
            // standby (its durationchange fired before it was active), so pull
            // the timeline straight off it instead of waiting for a new event.
            currentTime.value = activeEl?.currentTime || 0
            duration.value = activeEl?.duration || 0
            play()
            updatePreload()
            return
        }

        loadTrack(nextIndex)
        play()
    }

    const playPrevious = (): void => {
        if (currentTime.value > 3) {
            if (activeEl) activeEl.currentTime = 0
            return
        }

        let prevIndex = currentIndex.value - 1

        if (prevIndex < 0) {
            if (repeat.value === 'all') {
                prevIndex = queue.value.length - 1
            } else {
                if (activeEl) activeEl.currentTime = 0
                return
            }
        }

        loadTrack(prevIndex)
        play()
    }

    const seek = (time: number): void => {
        if (activeEl) activeEl.currentTime = time
        currentTime.value = time
    }

    const setVolume = (newVolume: number): void => {
        volume.value = Math.max(0, Math.min(1, newVolume))
    }

    const toggleRepeat = (): void => {
        const modes: Array<'none' | 'all' | 'one'> = ['none', 'all', 'one']
        const currentModeIndex = modes.indexOf(repeat.value)
        const nextMode = modes[(currentModeIndex + 1) % modes.length]
        if (nextMode) {
            repeat.value = nextMode
        }
    }

    const toggleShuffle = (): void => {
        shuffle.value = !shuffle.value
    }

    const addToQueue = (song: Song): void => {
        queue.value.push(song)
        updatePreload()
    }

    const addMultipleToQueue = (songs: Song[]): void => {
        queue.value.push(...songs)
        updatePreload()
    }

    const playNow = (song: Song): void => {
        queue.value = [song]
        loadTrack(0)
        play()
    }

    const playAlbum = (songs: Song[], startIndex: number = 0): void => {
        queue.value = [...songs]
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
        updatePreload()
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
            if (wasPlaying) play()
        } else if (current) {
            // Keep pointing at the still-playing track.
            const idx = next.indexOf(current)
            if (idx !== -1) currentIndex.value = idx
            updatePreload()
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
        updatePreload()
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
        updatePreload()
    }

    const clearQueue = (): void => {
        queue.value = []
        currentIndex.value = 0
        currentTrack.value = null
        preloadedTrack.value = null
        preloadedUrl = null
        pause()
        if (activeEl) activeEl.src = ''
        if (standbyEl) standbyEl.src = ''
    }

    const playQueueItem = (index: number): void => {
        loadTrack(index)
        play()
    }

    playNextFn = playNext

    return {
        playerState,
        currentTrack,
        queue,
        currentIndex,
        isPlaying,
        volume,
        repeat,
        shuffle,
        currentTime,
        duration,
        preloadedTrack,
        hasNext,
        hasPrevious,
        play,
        pause,
        togglePlayPause,
        playNext,
        playPrevious,
        seek,
        setVolume,
        toggleRepeat,
        toggleShuffle,
        addToQueue,
        addMultipleToQueue,
        playNow,
        playAlbum,
        removeFromQueue,
        removeManyFromQueue,
        moveInQueue,
        insertIntoQueue,
        clearQueue,
        playQueueItem
    }
}
