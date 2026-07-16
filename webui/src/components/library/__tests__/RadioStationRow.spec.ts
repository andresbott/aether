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

import { RouterLinkStub } from '@vue/test-utils'
import RadioStationRow from '@/components/library/RadioStationRow.vue'
import type { InternetRadioStation } from '@/types/subsonic'

const station: InternetRadioStation = {
    id: 's1',
    name: 'Jazz FM',
    streamUrl: 'http://stream/jazz',
    homepageUrl: 'http://jazzfm.example',
    coverArt: 'ca1'
}

const mountRow = (s?: InternetRadioStation) =>
    mount(RadioStationRow, {
        props: { station: s },
        global: { stubs: { RouterLink: RouterLinkStub } }
    })

describe('RadioStationRow', () => {
    it('renders avatar (size 80), name and homepage', () => {
        const w = mountRow(station)
        expect(w.find('img').attributes('src')).toBe('cover:ca1:80')
        expect(w.find('.col-name').text()).toBe('Jazz FM')
        expect(w.find('.col-homepage').text()).toBe('http://jazzfm.example')
    })

    it('links to the station detail route', () => {
        const w = mountRow(station)
        expect(w.findComponent(RouterLinkStub).props('to')).toEqual({
            name: 'radio-station-detail',
            params: { id: 's1' }
        })
    })

    it('starts a songs drag carrying the station as a single song', async () => {
        const w = mountRow(station)
        await w.find('.radio-row').trigger('dragstart')
        expect(start).toHaveBeenCalledTimes(1)
        const songs = start.mock.calls[0][1]
        expect(songs).toHaveLength(1)
        expect(songs[0]).toMatchObject({ title: 'Jazz FM', streamUrl: 'http://stream/jazz' })
    })

    it('renders a placeholder row when the station is not loaded', () => {
        const w = mountRow(undefined)
        expect(w.find('.radio-row.placeholder').exists()).toBe(true)
        expect(w.find('img').exists()).toBe(false)
    })
})
