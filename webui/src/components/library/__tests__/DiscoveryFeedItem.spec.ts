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

    it('renders a PlaylistCard for a playlist', () => {
        const w = mountItem(playlistEntry())
        expect(w.find('.stub-playlist-card').exists()).toBe(true)
        expect(w.find('.stub-album-card').exists()).toBe(false)
    })

    it('shows a human label for each reason', () => {
        const cases: Array<[string, string]> = [
            ['favorite', 'Favorite'],
            ['recentlyAdded', 'Recently added'],
            ['mostPlayed', 'Most played'],
            ['recentlyPlayed', 'Recently played'],
            ['genreMatch', 'Your genres'],
            ['rediscover', 'Rediscover']
        ]
        for (const [reason, label] of cases) {
            const w = mountItem(albumEntry(reason))
            expect(w.find('.discovery-reason-badge').text()).toBe(label)
        }
    })

    it('exposes the stable hook class', () => {
        expect(mountItem(albumEntry()).find('.discovery-feed-item').exists()).toBe(true)
    })
})
