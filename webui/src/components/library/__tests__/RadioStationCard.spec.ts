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

import RadioStationCard from '@/components/library/RadioStationCard.vue'
import type { InternetRadioStation } from '@/types/subsonic'

const station: InternetRadioStation = {
    id: 's1',
    name: 'Jazz FM',
    streamUrl: 'http://stream/jazz',
    homepageUrl: 'http://jazzfm.example',
    coverArt: 'ca1'
}

const mountCard = (s?: InternetRadioStation) => mount(RadioStationCard, { props: { station: s } })

describe('RadioStationCard', () => {
    it('renders the cover, name and homepage subtitle', () => {
        const w = mountCard(station)
        expect(w.find('img').attributes('src')).toBe('cover:ca1:200')
        expect(w.find('.card-title').text()).toBe('Jazz FM')
        expect(w.find('.card-subtitle').text()).toBe('http://jazzfm.example')
    })

    it('does not play the station on click', async () => {
        const w = mountCard(station)
        await w.find('.radio-card').trigger('click')
        // Nothing to assert on a player — clicking is a no-op; the station is
        // added to the queue only by dragging it there.
        expect(start).not.toHaveBeenCalled()
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
