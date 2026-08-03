import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useToggleStar } from '@/composables/useSubsonicQueries'
import type { Song } from '@/types/subsonic'

/**
 * The favorite state of one song, plus the toggle that flips it. This is the
 * single place a track's `starred` is read and written, shared by the player
 * bar's heart, the `L` shortcut and every track row.
 *
 * It does two things on a toggle, because songs reach the UI two ways:
 *
 * - **Mutates `song.starred` locally.** The play queue is plain reactive state
 *   (`usePlayer`, localStorage-backed), not a query, so nothing would otherwise
 *   refresh it until a reload.
 * - **Lets `useToggleStar` invalidate `['subsonic']`.** Query-backed rows (album
 *   detail, search, genre detail) get their authoritative value from the refetch;
 *   the local flip just makes the heart respond on the same tick.
 *
 * The optimistic write is safe to lose: `starred` is only ever a presence check
 * in this app, and the server is the source of truth a moment later.
 */
export function useSongFavorite(song: MaybeRefOrGetter<Song | null | undefined>) {
    const toggleStar = useToggleStar()

    const isStarred = computed(() => !!toValue(song)?.starred)

    const toggleFavorite = (): void => {
        const s = toValue(song)
        if (!s) return
        // `useToggleStar` takes the CURRENT state and flips it — passing `true`
        // unstars.
        toggleStar.mutate({ id: s.id, starred: isStarred.value })
        s.starred = isStarred.value ? undefined : new Date().toISOString()
    }

    return { isStarred, toggleFavorite }
}
