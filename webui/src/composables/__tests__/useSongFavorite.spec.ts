import { describe, it, expect, vi, beforeEach } from 'vitest'
import { reactive, ref } from 'vue'

const starMutate = vi.fn()
vi.mock('@/composables/useSubsonicQueries', () => ({
    useToggleStar: () => ({ mutate: starMutate })
}))

import { useSongFavorite } from '@/composables/useSongFavorite'
import type { Song } from '@/types/subsonic'

const song = (over: Partial<Song> = {}): Song =>
    ({ id: 's1', title: 'One', artist: 'A', ...over }) as Song

beforeEach(() => {
    starMutate.mockReset()
})

describe('useSongFavorite', () => {
    it('reads starred as a presence check', () => {
        expect(useSongFavorite(song()).isStarred.value).toBe(false)
        expect(useSongFavorite(song({ starred: '2026-02-01T00:00:00Z' })).isStarred.value).toBe(
            true
        )
    })

    it('passes the CURRENT state to the mutation, which flips it', () => {
        useSongFavorite(song()).toggleFavorite()
        expect(starMutate).toHaveBeenCalledWith({ id: 's1', starred: false })

        starMutate.mockReset()
        useSongFavorite(song({ starred: '2026-02-01T00:00:00Z' })).toggleFavorite()
        expect(starMutate).toHaveBeenCalledWith({ id: 's1', starred: true })
    })

    // The play queue is plain reactive state, not query-backed, so without this
    // local write a queue row's heart would not move until a reload. The song is
    // `reactive` here because that is how it reaches a row in the app — a member
    // of the player's reactive queue — and only then does the flip re-render.
    it('optimistically stamps starred on the song object', () => {
        const s = reactive(song())
        const { isStarred, toggleFavorite } = useSongFavorite(s)
        toggleFavorite()
        expect(typeof s.starred).toBe('string')
        expect(isStarred.value).toBe(true)
    })

    it('optimistically clears starred when unstarring', () => {
        const s = reactive(song({ starred: '2026-02-01T00:00:00Z' }))
        const { isStarred, toggleFavorite } = useSongFavorite(s)
        toggleFavorite()
        expect(s.starred).toBeUndefined()
        expect(isStarred.value).toBe(false)
    })

    // Rows must receive the queue's own song object, not a `{ ...song }` copy:
    // the optimistic write would land on the throwaway and the heart would snap
    // back on the next recompute. This is what QueueBody's row mapping guards.
    it('writes through to the queue member a row was handed', () => {
        const queue = reactive([song(), song({ id: 's2' })])
        const { toggleFavorite } = useSongFavorite(() => queue[1])
        toggleFavorite()
        expect(typeof queue[1].starred).toBe('string')
        expect(queue[0].starred).toBeUndefined()
    })

    it('tracks a ref source, so a changing current track is followed', () => {
        const source = ref<Song | null>(null)
        const { isStarred } = useSongFavorite(source)
        expect(isStarred.value).toBe(false)
        source.value = song({ starred: '2026-02-01T00:00:00Z' })
        expect(isStarred.value).toBe(true)
    })

    it('tracks a getter source, so a row following its prop is followed', () => {
        const holder = ref<Song | undefined>(undefined)
        const { isStarred } = useSongFavorite(() => holder.value)
        expect(isStarred.value).toBe(false)
        holder.value = song({ starred: '2026-02-01T00:00:00Z' })
        expect(isStarred.value).toBe(true)
    })

    it('is a no-op with no song', () => {
        const { toggleFavorite, isStarred } = useSongFavorite(null)
        toggleFavorite()
        expect(starMutate).not.toHaveBeenCalled()
        expect(isStarred.value).toBe(false)
    })
})
