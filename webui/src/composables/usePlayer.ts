import { ref, computed, watch } from 'vue'
import type { Song, PlayerState } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { saveToLocalStorage, loadFromLocalStorage } from '@/utils/localStorage'
import { reorderQueue } from '@/utils/queueReorder'

const STORAGE_KEY_QUEUE = 'musicPlayer:queue'
const STORAGE_KEY_CURRENT_INDEX = 'musicPlayer:currentIndex'
const STORAGE_KEY_VOLUME = 'musicPlayer:volume'
const STORAGE_KEY_REPEAT = 'musicPlayer:repeat'
const STORAGE_KEY_SHUFFLE = 'musicPlayer:shuffle'

const currentTrack = ref<Song | null>(null)
const queue = ref<Song[]>(loadFromLocalStorage<Song[]>(STORAGE_KEY_QUEUE, []))
const currentIndex = ref<number>(
    loadFromLocalStorage<number>(STORAGE_KEY_CURRENT_INDEX, 0)
)
const isPlaying = ref<boolean>(false)
const volume = ref<number>(loadFromLocalStorage<number>(STORAGE_KEY_VOLUME, 1))
const repeat = ref<'none' | 'all' | 'one'>(
    loadFromLocalStorage<'none' | 'all' | 'one'>(STORAGE_KEY_REPEAT, 'none')
)
const shuffle = ref<boolean>(loadFromLocalStorage<boolean>(STORAGE_KEY_SHUFFLE, false))
const currentTime = ref<number>(0)
const duration = ref<number>(0)

let audioElement: HTMLAudioElement | null = null
let playNextFn: (() => void) | null = null

const initAudioElement = (): HTMLAudioElement => {
    if (!audioElement) {
        audioElement = new Audio()
        audioElement.addEventListener('timeupdate', () => {
            currentTime.value = audioElement?.currentTime || 0
        })
        audioElement.addEventListener('durationchange', () => {
            duration.value = audioElement?.duration || 0
        })
        audioElement.addEventListener('ended', () => {
            if (playNextFn) playNextFn()
        })
        audioElement.addEventListener('play', () => {
            isPlaying.value = true
        })
        audioElement.addEventListener('pause', () => {
            isPlaying.value = false
        })
    }
    return audioElement
}

const getTrackUrl = (track: Song | null): string | null => {
    if (!track) return null
    if (track.streamUrl) return track.streamUrl
    if (!subsonicClient.isConfigured()) return null
    return subsonicClient.getStreamUrl(track.id)
}

export function usePlayer() {
    const audio = initAudioElement()

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

    if (!audio.src) {
        const url = getTrackUrl(currentTrack.value)
        if (url) {
            audio.src = url
            audio.volume = volume.value
        }
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
        audio.volume = newVolume
        saveToLocalStorage(STORAGE_KEY_VOLUME, newVolume)
    })

    const play = (): void => {
        if (!currentTrack.value && queue.value.length > 0) {
            loadTrack(0)
        }
        if (!audio.src) return
        audio.play().catch((err) => {
            if (err.name !== 'AbortError') {
                console.error('Failed to play:', err)
            }
        })
    }

    const pause = (): void => {
        audio.pause()
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
        if (url) {
            audio.src = url
            audio.volume = volume.value
        }
    }

    const playNext = (): void => {
        if (repeat.value === 'one' && currentTrack.value) {
            audio.currentTime = 0
            play()
            return
        }

        let nextIndex = currentIndex.value + 1

        if (nextIndex >= queue.value.length) {
            if (repeat.value === 'all') {
                nextIndex = 0
            } else {
                pause()
                return
            }
        }

        loadTrack(nextIndex)
        play()
    }

    const playPrevious = (): void => {
        if (currentTime.value > 3) {
            audio.currentTime = 0
            return
        }

        let prevIndex = currentIndex.value - 1

        if (prevIndex < 0) {
            if (repeat.value === 'all') {
                prevIndex = queue.value.length - 1
            } else {
                audio.currentTime = 0
                return
            }
        }

        loadTrack(prevIndex)
        play()
    }

    const seek = (time: number): void => {
        audio.currentTime = time
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
    }

    const addMultipleToQueue = (songs: Song[]): void => {
        queue.value.push(...songs)
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
        queue.value = [
            ...queue.value.slice(0, clamped),
            ...songs,
            ...queue.value.slice(clamped)
        ]
        if (current) {
            const idx = queue.value.indexOf(current)
            if (idx !== -1) currentIndex.value = idx
        }
    }

    const clearQueue = (): void => {
        queue.value = []
        currentIndex.value = 0
        currentTrack.value = null
        pause()
        audio.src = ''
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
        moveInQueue,
        insertIntoQueue,
        clearQueue,
        playQueueItem
    }
}
