import { onBeforeUnmount, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueSidebar } from '@/composables/useQueueSidebar'
import { useCurrentTrackFavorite } from '@/composables/useCurrentTrackFavorite'
import { useShortcutHelp } from '@/composables/useShortcutHelp'
import { resolveShortcutAction, isTypingTarget, type ShortcutAction } from '@/utils/shortcuts'

const SEEK_STEP_SECONDS = 5
const VOLUME_STEP = 0.05

// Volume is a 0..1 float, so a run of +/- presses accumulates binary drift
// (0.5 + 0.05 = 0.5500000000000001) that then shows up in localStorage and in
// the rail's position. Round to whole percent, which is also the rail's own
// granularity.
const stepVolume = (current: number, delta: number): number =>
    Math.min(1, Math.max(0, Math.round((current + delta) * 100) / 100))

// Bound once from PlayerLayout: the player bar is the only place these actions
// make sense, and the settings layout deliberately gets no bindings.
export function useKeyboardShortcuts(): void {
    const player = usePlayer()
    const { toggleSidebar } = useQueueSidebar()
    const { toggleFavorite } = useCurrentTrackFavorite()
    const help = useShortcutHelp()
    const router = useRouter()

    const seekBy = (delta: number): boolean => {
        if (!player.duration.value) return false
        const target = player.currentTime.value + delta
        player.seek(Math.min(player.duration.value, Math.max(0, target)))
        return true
    }

    // Returns whether the press was consumed. Anything not consumed falls
    // through to the browser untouched.
    const run = (action: ShortcutAction): boolean => {
        switch (action) {
            case 'play-pause':
                player.togglePlayPause()
                return true
            case 'next':
                player.playNext()
                return true
            case 'previous':
                player.playPrevious()
                return true
            case 'seek-forward':
                return seekBy(SEEK_STEP_SECONDS)
            case 'seek-back':
                return seekBy(-SEEK_STEP_SECONDS)
            case 'volume-up':
                player.setVolume(stepVolume(player.volume.value, VOLUME_STEP))
                return true
            case 'volume-down':
                player.setVolume(stepVolume(player.volume.value, -VOLUME_STEP))
                return true
            case 'mute':
                player.toggleMute()
                return true
            case 'favorite':
                toggleFavorite()
                return true
            case 'queue':
                toggleSidebar()
                return true
            case 'search':
                void router.push({ name: 'search' })
                return true
            // Now Playing is the `home` route — HomeView renders QueueView at `/`.
            case 'now-playing':
                void router.push({ name: 'home' })
                return true
            // No folderId: the cross-collection library root, which opens on the
            // Discover feed. Passing one would scope it to a single library.
            case 'library':
                void router.push({ name: 'library' })
                return true
            case 'playlists':
                void router.push({ name: 'playlists' })
                return true
            case 'genres':
                void router.push({ name: 'genres' })
                return true
            case 'radio':
                void router.push({ name: 'radio' })
                return true
            case 'help':
                help.toggle()
                return true
            case 'close':
                // Only claim Escape while the overlay is up; otherwise it belongs
                // to whatever dialog or edit mode is listening for it.
                if (!help.open.value) return false
                help.close()
                return true
        }
    }

    const onKeydown = (event: KeyboardEvent): void => {
        if (isTypingTarget(event.target)) return
        const action = resolveShortcutAction(event)
        if (!action) return
        if (!run(action)) return
        // Space and the arrows scroll the page: a handled key must not also reach
        // the browser.
        event.preventDefault()
    }

    onMounted(() => document.addEventListener('keydown', onKeydown))
    onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
}
