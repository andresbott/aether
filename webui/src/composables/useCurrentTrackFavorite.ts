import { computed } from 'vue'
import { usePlayer } from '@/composables/usePlayer'
import { useToggleStar } from '@/composables/useSubsonicQueries'

// The one favorite affordance for whatever is playing, shared by the player
// bar's heart and the `L` shortcut so both flip the same state the same way.
export function useCurrentTrackFavorite() {
    const player = usePlayer()
    const toggleStar = useToggleStar()

    const isStarred = computed(() => !!player.currentTrack.value?.starred)

    const toggleFavorite = (): void => {
        const track = player.currentTrack.value
        if (!track) return
        toggleStar.mutate({ id: track.id, starred: isStarred.value })
        // Optimistic local flip so the heart updates immediately (currentTrack
        // isn't query-backed, so it wouldn't otherwise reflect the change until
        // reload).
        track.starred = isStarred.value ? undefined : new Date().toISOString()
    }

    return { isStarred, toggleFavorite }
}
