import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import type { Playlist } from '@/types/subsonic'

const { starMutate, scrobbleMock, getPlaylistMock, playAlbumMock, isConfiguredMock, getCoverArtUrlMock } =
    vi.hoisted(() => ({
        starMutate: vi.fn(),
        scrobbleMock: vi.fn(() => Promise.resolve()),
        getPlaylistMock: vi.fn(() => Promise.resolve({ entry: [{ id: 'tr-1' }] })),
        playAlbumMock: vi.fn(),
        isConfiguredMock: vi.fn(() => false),
        getCoverArtUrlMock: vi.fn((_id: string, _size?: number): string => '')
    }))

vi.mock('@/composables/useSubsonicQueries', () => ({
    useTogglePlaylistStar: () => ({ mutate: starMutate })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: isConfiguredMock,
        getCoverArtUrl: getCoverArtUrlMock,
        getPlaylist: getPlaylistMock,
        scrobble: scrobbleMock
    }
}))

vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playAlbum: playAlbumMock }) }))

// null = auth "none" (no per-user identity) → treated as owner, no marker.
const currentUser = ref<{ login: string; role: string } | null>(null)
vi.mock('@/composables/useAuth', () => ({ useAuth: () => ({ currentUser }) }))

import { bumpCoverVersion, resetCoverVersions } from '@/composables/useCoverVersion'
import PlaylistCard from '@/components/library/PlaylistCard.vue'

const playlist = (over: Partial<Playlist> = {}): Playlist => ({
    id: 'pl-1',
    name: 'Mix',
    songCount: 3,
    duration: 300,
    created: '2026-01-01T00:00:00Z',
    ...over
})

const stubs = { RouterLink: { template: '<a><slot /></a>' } }
const mountCard = (pl: Playlist) =>
    mount(PlaylistCard, { props: { playlist: pl }, global: { stubs } })

beforeEach(() => {
    starMutate.mockReset()
    scrobbleMock.mockClear()
    playAlbumMock.mockReset()
    currentUser.value = null
    isConfiguredMock.mockReturnValue(false)
    getCoverArtUrlMock.mockReturnValue('')
    resetCoverVersions()
})

describe('PlaylistCard star toggle', () => {
    it('shows an outline heart when unstarred and a filled one when starred', () => {
        expect(mountCard(playlist()).find('.card-star i').classes()).toContain('pi-heart')
        const starred = mountCard(playlist({ starred: '2026-02-01T00:00:00Z' }))
        expect(starred.find('.card-star i').classes()).toContain('pi-heart-fill')
    })

    it('labels the favorite toggle for screen readers', () => {
        expect(mountCard(playlist()).find('.card-star').attributes('aria-label')).toBe(
            'Add to favorites'
        )
        expect(
            mountCard(playlist({ starred: '2026-02-01T00:00:00Z' }))
                .find('.card-star')
                .attributes('aria-label')
        ).toBe('Remove from favorites')
    })

    it('keeps a starred playlist star visible without hover', () => {
        expect(mountCard(playlist({ starred: '2026-02-01T00:00:00Z' })).find('.card-star').classes())
            .toContain('is-starred')
        expect(mountCard(playlist()).find('.card-star').classes()).not.toContain('is-starred')
    })

    it('toggles the star with the playlist id and its current state', async () => {
        const w = mountCard(playlist({ starred: '2026-02-01T00:00:00Z' }))
        await w.find('.card-star').trigger('click')
        expect(starMutate).toHaveBeenCalledWith({ id: 'pl-1', starred: true })
    })

    it('scrobbles the playlist when it is played', async () => {
        const w = mountCard(playlist())
        await w.find('.card-play').trigger('click')
        await Promise.resolve()
        await Promise.resolve()
        expect(scrobbleMock).toHaveBeenCalledWith('pl-1')
        expect(playAlbumMock).toHaveBeenCalled()
    })
})

describe('PlaylistCard ownership marker', () => {
    it('marks a playlist owned by someone else with a read-only icon after the song count', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        expect(
            mountCard(playlist({ owner: 'bob' })).find('.card-subtitle .not-mine-icon').exists()
        ).toBe(true)
    })

    it('shows no marker on your own playlist', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        expect(mountCard(playlist({ owner: 'alice' })).find('.not-mine-icon').exists()).toBe(false)
    })

    it('shows no marker with no identity (auth "none")', () => {
        currentUser.value = null
        expect(mountCard(playlist({ owner: 'admin' })).find('.not-mine-icon').exists()).toBe(false)
    })
})

describe('PlaylistCard cover cache busting', () => {
    it('appends the cover version so an edited cover refreshes without a reload', async () => {
        isConfiguredMock.mockReturnValue(true)
        getCoverArtUrlMock.mockImplementation((id: string) => `/rest/getCoverArt.view?id=${id}`)
        const w = mountCard(playlist({ coverArt: 'pl-cover' }))
        // Before any edit the url is un-versioned.
        expect(w.find('.card-cover img').attributes('src')).not.toContain('_v=')
        // Editing the cover elsewhere bumps the module-level version; the card,
        // which renders the same coverArt id, must pick it up (the bug was that
        // it rendered a plain, cached url and stayed stale until reload).
        bumpCoverVersion('pl-cover')
        await w.vm.$nextTick()
        expect(w.find('.card-cover img').attributes('src')).toContain('_v=1')
    })
})
