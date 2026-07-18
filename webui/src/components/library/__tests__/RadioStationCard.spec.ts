import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'

const start = vi.fn()
const end = vi.fn()
vi.mock('@/composables/useSongsDrag', () => ({ useSongsDrag: () => ({ start, end }) }))
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (art: string, size: number) => `cover:${art}:${size}`
    }
}))
const playNow = vi.fn()
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playNow }) }))
vi.mock('@/utils/radioSong', () => ({ stationToSong: (s: any) => ({ id: s.id, title: s.name, streamUrl: s.streamUrl }) }))

import { RouterLinkStub } from '@vue/test-utils'
import RadioStationCard from '@/components/library/RadioStationCard.vue'
import type { InternetRadioStation } from '@/types/subsonic'

const station: InternetRadioStation = {
    id: 's1',
    name: 'Jazz FM',
    streamUrl: 'http://stream/jazz',
    homepageUrl: 'http://jazzfm.example',
    coverArt: 'ca1'
}

const mountCard = (s?: InternetRadioStation) =>
    mount(RadioStationCard, {
        props: { station: s },
        global: { stubs: { RouterLink: RouterLinkStub } }
    })

describe('RadioStationCard', () => {
    it('renders the cover, name and homepage subtitle', () => {
        const w = mountCard(station)
        expect(w.find('img').attributes('src')).toBe('cover:ca1:200')
        expect(w.find('.card-title').text()).toBe('Jazz FM')
        expect(w.find('.card-subtitle').text()).toBe('http://jazzfm.example')
    })

    it('links to the station detail route', () => {
        const w = mountCard(station)
        expect(w.findComponent(RouterLinkStub).props('to')).toEqual({
            name: 'radio-station-detail',
            params: { id: 's1' }
        })
    })

    it('the play button plays the station without navigating', async () => {
        const w = mountCard(station)
        await w.find('.card-play').trigger('click')
        expect(playNow).toHaveBeenCalledTimes(1)
    })

    it('starts a songs drag carrying the station as a single song', async () => {
        const w = mountCard(station)
        await w.find('.radio-card').trigger('dragstart')
        expect(start).toHaveBeenCalledTimes(1)
        const songs = start.mock.calls[0][1]
        expect(songs).toHaveLength(1)
        expect(songs[0]).toMatchObject({ title: 'Jazz FM', streamUrl: 'http://stream/jazz' })
    })

    it('is draggable with a non-draggable cover image', () => {
        const w = mountCard(station)
        expect(w.find('.radio-card').attributes('draggable')).toBe('true')
        expect(w.find('img').attributes('draggable')).toBe('false')
    })

    it('renders a placeholder with no image when the station is undefined', () => {
        const w = mountCard(undefined)
        expect(w.find('.radio-card.placeholder').exists()).toBe(true)
        expect(w.find('img').exists()).toBe(false)
    })
})
