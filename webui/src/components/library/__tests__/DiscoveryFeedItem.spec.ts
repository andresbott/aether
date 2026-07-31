import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { DiscoveryFeedEntry } from '@/types/subsonic'

vi.mock('@/components/library/AlbumCard.vue', () => ({
    default: { name: 'AlbumCard', props: ['album'], template: '<div class="stub-album-card" />' }
}))
vi.mock('@/components/library/AlbumRow.vue', () => ({
    default: { name: 'AlbumRow', props: ['album'], template: '<div class="stub-album-row" />' }
}))
vi.mock('@/components/library/PlaylistCard.vue', () => ({
    default: {
        name: 'PlaylistCard',
        props: ['playlist'],
        template: '<div class="stub-playlist-card" />'
    }
}))
vi.mock('@/components/library/PlaylistRow.vue', () => ({
    default: {
        name: 'PlaylistRow',
        props: ['playlist'],
        template: '<div class="stub-playlist-row" />'
    }
}))

import DiscoveryFeedItem from '@/components/library/DiscoveryFeedItem.vue'

const albumEntry = (reason = 'favorite'): DiscoveryFeedEntry => ({
    type: 'album',
    rank: 0,
    reason: reason as DiscoveryFeedEntry['reason'],
    album: { id: 'al-1', name: 'Album', rank: 0, reason: reason as never }
})

const playlistEntry = (reason = 'rediscover'): DiscoveryFeedEntry => ({
    type: 'playlist',
    rank: 1,
    reason: reason as DiscoveryFeedEntry['reason'],
    playlist: {
        id: 'pl-1',
        name: 'PL',
        songCount: 1,
        duration: 1,
        created: '2026-01-01T00:00:00Z',
        rank: 1,
        reason: reason as never
    }
})

const mountItem = (entry: DiscoveryFeedEntry, layout: 'grid' | 'list' = 'grid') =>
    mount(DiscoveryFeedItem, { props: { entry, layout } })

describe('DiscoveryFeedItem', () => {
    it('renders an AlbumCard for an album in grid layout', () => {
        const w = mountItem(albumEntry())
        expect(w.find('.stub-album-card').exists()).toBe(true)
        expect(w.find('.stub-album-row').exists()).toBe(false)
    })

    it('renders an AlbumRow for an album in list layout', () => {
        const w = mountItem(albumEntry(), 'list')
        expect(w.find('.stub-album-row').exists()).toBe(true)
        expect(w.find('.stub-album-card').exists()).toBe(false)
    })

    it('renders a PlaylistCard for a playlist in grid layout', () => {
        const w = mountItem(playlistEntry())
        expect(w.find('.stub-playlist-card').exists()).toBe(true)
        expect(w.find('.stub-playlist-row').exists()).toBe(false)
        expect(w.find('.stub-album-card').exists()).toBe(false)
    })

    // A playlist rendered as a CARD inside a list would tower over the album rows
    // beside it, which is the bug PlaylistRow exists to prevent.
    it('renders a PlaylistRow for a playlist in list layout', () => {
        const w = mountItem(playlistEntry(), 'list')
        expect(w.find('.stub-playlist-row').exists()).toBe(true)
        expect(w.find('.stub-playlist-card').exists()).toBe(false)
    })

    // The reason is still served by the API (other clients may use it) but is
    // deliberately not rendered: on a lightly-played library nearly every item
    // carries the same reason, so the badge was noise rather than information.
    it('renders no reason badge and no reason text', () => {
        const reasons = [
            'favorite',
            'recentlyAdded',
            'mostPlayed',
            'recentlyPlayed',
            'genreMatch',
            'rediscover'
        ]
        const labels = [
            'Favorite',
            'Recently added',
            'Most played',
            'Recently played',
            'Your genres',
            'Rediscover'
        ]
        for (const reason of reasons) {
            for (const layout of ['grid', 'list'] as const) {
                const w = mountItem(albumEntry(reason), layout)
                expect(w.find('.discovery-reason-badge').exists()).toBe(false)
                for (const label of labels) {
                    expect(w.text()).not.toContain(label)
                }
            }
        }
    })

    it('puts the stable hook class on the root element', () => {
        const w = mountItem(albumEntry())
        // classList on the root, not find() over the subtree: find() would pass
        // even if the class had drifted onto a child, and Task 10 targets the root.
        expect(w.element.classList.contains('discovery-feed-item')).toBe(true)
    })

    it('carries the layout as a modifier class on the root', () => {
        expect(mountItem(albumEntry(), 'grid').element.classList.contains('grid')).toBe(true)
        expect(mountItem(albumEntry(), 'list').element.classList.contains('list')).toBe(true)
    })
})
