import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { WishlistKind, WishlistSearchResult } from '@/types/wishlist'

const searchWishlist = vi.fn()
vi.mock('@/lib/api/Wishlist', () => ({ searchWishlist: (...a: unknown[]) => searchWishlist(...a) }))
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

import { useWishlistSearch } from '@/composables/useWishlist'

const hit = (mbid: string, kind: WishlistKind = 'album'): WishlistSearchResult => ({
    kind,
    mbid,
    title: mbid,
    artist: '',
    album: '',
    date: '',
    type: '',
    disambiguation: '',
    score: 100,
    in_library: false
})

function deferred<T>() {
    let resolve!: (v: T) => void
    const promise = new Promise<T>((r) => (resolve = r))
    return { promise, resolve }
}

const kindsAsked = () => searchWishlist.mock.calls.map((c) => (c[0] as { kind: string }).kind)

// Braced: a function returned from beforeEach is run as a cleanup hook, and
// mockReset returns the mock itself.
beforeEach(() => {
    searchWishlist.mockReset()
})

describe('useWishlistSearch', () => {
    it('fetches each asked kind and files the answers by kind', async () => {
        searchWishlist.mockImplementation(async ({ kind }: { kind: WishlistKind }) => [hit(`${kind}-1`, kind)])
        const s = useWishlistSearch()
        await s.search(' nevermind ', ['artist', 'album', 'song'])
        expect(kindsAsked()).toEqual(['artist', 'album', 'song'])
        expect(searchWishlist.mock.calls[0][0]).toEqual({ kind: 'artist', q: 'nevermind' })
        expect(s.term.value).toBe('nevermind')
        expect(s.results.value.album?.[0].mbid).toBe('album-1')
        expect(s.loading.value).toBe(false)
    })

    it('reuses a kind already fetched for the same text, and fetches only what is missing', async () => {
        searchWishlist.mockResolvedValue([hit('x')])
        const s = useWishlistSearch()
        await s.search('nevermind', ['artist', 'album', 'song'])
        await s.search('nevermind', ['album'])
        expect(searchWishlist).toHaveBeenCalledTimes(3)

        searchWishlist.mockClear()
        await s.search('in utero', ['album'])
        await s.search('in utero', ['album', 'song'])
        expect(kindsAsked()).toEqual(['album', 'song'])
    })

    it('drops an old text’s answer that lands after a new search', async () => {
        const old = deferred<WishlistSearchResult[]>()
        searchWishlist.mockReturnValueOnce(old.promise).mockResolvedValueOnce([hit('new')])
        const s = useWishlistSearch()

        const a = s.search('never', ['album'])
        const b = s.search('nevermind', ['album'])
        expect((searchWishlist.mock.calls[0][1] as AbortSignal).aborted).toBe(true)
        await b
        old.resolve([hit('old')])
        await a
        expect(s.results.value.album?.map((r) => r.mbid)).toEqual(['new'])
        expect(s.loading.value).toBe(false)
    })

    it('does not search a too-short text', async () => {
        const s = useWishlistSearch()
        await s.search(' a ', ['album'])
        expect(searchWishlist).not.toHaveBeenCalled()
        expect(s.term.value).toBe('')
    })

    it('reports a rate limit, keeps the kinds that answered, and retries the failed one', async () => {
        searchWishlist.mockImplementation(async ({ kind }: { kind: WishlistKind }) => {
            if (kind === 'song') throw { response: { status: 429, data: { detail: 'slow down' } } }
            return [hit(kind, kind)]
        })
        const s = useWishlistSearch()
        await s.search('nirvana', ['album', 'song'])
        expect(s.rateLimited.value).toBe(true)
        expect(s.error.value).toBeTruthy()
        expect(s.results.value.album).toHaveLength(1)
        expect('song' in s.results.value).toBe(false)

        searchWishlist.mockClear()
        searchWishlist.mockResolvedValue([hit('s', 'song')])
        await s.search('nirvana', ['album', 'song'])
        expect(kindsAsked()).toEqual(['song'])
        expect(s.error.value).toBeNull()
    })

    it('clear forgets the text and the results', async () => {
        searchWishlist.mockResolvedValue([hit('x')])
        const s = useWishlistSearch()
        await s.search('nevermind', ['album'])
        s.clear()
        expect(s.term.value).toBe('')
        expect(s.results.value).toEqual({})
    })
})
