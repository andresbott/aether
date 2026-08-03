import { usePlayer } from '@/composables/usePlayer'
import { useSongFavorite } from '@/composables/useSongFavorite'

// The one favorite affordance for whatever is playing, shared by the player
// bar's heart and the `L` shortcut so both flip the same state the same way.
// The mechanics (optimistic flip + mutation) live in `useSongFavorite`, which
// every track row also uses — this is just that composable bound to the
// currently playing track.
export function useCurrentTrackFavorite() {
    const player = usePlayer()
    return useSongFavorite(player.currentTrack)
}
