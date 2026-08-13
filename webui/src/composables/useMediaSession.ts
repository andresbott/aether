import { watch } from 'vue'
import { usePlayer } from '@/composables/usePlayer'
import { subsonicClient } from '@/lib/api/subsonic'

// Lock-screen / notification / hardware-key playback controls (spec §4.3).
// Bound once from PlayerLayout so every shell gets it — desktop gains
// hardware media keys from the same wiring. Everything is feature-detected
// and wrapped: an unsupported browser must never throw (spec §6).
let bound = false

const ARTWORK_SIZES = [96, 256, 512] as const

export function useMediaSession(): void {
    if (bound) return
    if (typeof navigator === 'undefined' || !('mediaSession' in navigator)) return
    bound = true

    const player = usePlayer()
    const session = navigator.mediaSession

    const syncMetadata = (): void => {
        const track = player.currentTrack.value
        if (!track) {
            session.metadata = null
            return
        }
        try {
            const artwork =
                track.coverArt && subsonicClient.isConfigured()
                    ? ARTWORK_SIZES.map((size) => ({
                          src: subsonicClient.getCoverArtUrl(track.coverArt as string, size),
                          sizes: `${size}x${size}`,
                          type: 'image/jpeg'
                      }))
                    : []
            session.metadata = new MediaMetadata({
                title: track.title ?? '',
                artist: track.artist ?? '',
                album: track.album ?? '',
                artwork
            })
        } catch {
            // MediaMetadata missing (jsdom) or artwork rejected: keep playing
            // without lock-screen art rather than surfacing an error.
        }
    }

    const syncPosition = (): void => {
        if (typeof session.setPositionState !== 'function') return
        const duration = player.duration.value
        if (!duration || !isFinite(duration)) return
        try {
            session.setPositionState({
                duration,
                playbackRate: 1,
                position: Math.min(player.currentTime.value, duration)
            })
        } catch {
            // Position state is advisory; a rejected update is not an error.
        }
    }

    watch(player.currentTrack, syncMetadata, { immediate: true })

    watch(
        player.isPlaying,
        (playing) => {
            session.playbackState = playing ? 'playing' : 'paused'
            syncPosition()
        },
        { immediate: true }
    )

    // Not on every currentTime tick: the browser extrapolates between updates.
    watch(player.duration, syncPosition)

    const on = (action: MediaSessionAction, handler: MediaSessionActionHandler): void => {
        try {
            session.setActionHandler(action, handler)
        } catch {
            // Browsers throw on actions they don't support; skip those.
        }
    }

    on('play', () => player.play())
    on('pause', () => player.pause())
    on('previoustrack', () => player.playPrevious())
    on('nexttrack', () => player.playNext())
    on('seekto', (details) => {
        if (details.seekTime != null) {
            player.seek(details.seekTime)
            syncPosition()
        }
    })
}

/** Test hook: allow a fresh bind with a new mediaSession stub. */
export function resetMediaSessionForTests(): void {
    bound = false
}
