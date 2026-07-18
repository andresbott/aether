import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { InternetRadioStation } from '@/types/subsonic'
import StationList from '@/views/settings/radio-stations/StationList.vue'

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { getCoverArtUrl: (id: string, size: number) => `/cover/${id}?s=${size}` }
}))

const stations: InternetRadioStation[] = [
    { id: 'r-1', name: 'BBC Radio 1', streamUrl: 'http://bbc/stream', coverArt: 'r-1' },
    { id: 'r-2', name: 'Jazz FM', streamUrl: 'http://jazz/stream' }
]

const mountList = (
    props: Partial<{
        stations: InternetRadioStation[]
        selectedId: string | null
        isLoading: boolean
    }> = {}
) =>
    mount(StationList, {
        props: { stations, selectedId: null, isLoading: false, ...props }
    })

describe('StationList', () => {
    it('renders a row per station with its name and stream URL', () => {
        const w = mountList()
        const rows = w.findAll('.station-row')
        expect(rows).toHaveLength(2)
        expect(w.text()).toContain('BBC Radio 1')
        expect(w.text()).toContain('Jazz FM')
        expect(w.text()).toContain('http://bbc/stream')
    })

    it('marks the selected station row', () => {
        const w = mountList({ selectedId: 'r-2' })
        const rows = w.findAll('.station-row')
        expect(rows[0].classes()).not.toContain('selected')
        expect(rows[1].classes()).toContain('selected')
    })

    it('emits select with the station when a row is clicked', async () => {
        const w = mountList()
        await w.findAll('.station-row')[0].trigger('click')
        expect(w.emitted('select')?.[0]?.[0]).toEqual(stations[0])
    })

    it('shows a cover thumbnail when the station has cover art', () => {
        const w = mountList()
        const img = w.findAll('.station-row')[0].find('img')
        expect(img.exists()).toBe(true)
        expect(img.attributes('src')).toContain('/cover/r-1')
    })

    it('shows an empty state when there are no stations', () => {
        const w = mountList({ stations: [] })
        expect(w.findAll('.station-row')).toHaveLength(0)
        expect(w.text()).toContain('No radio stations')
    })

    it('shows a loading indicator while loading', () => {
        const w = mountList({ stations: [], isLoading: true })
        expect(w.find('.loading').exists()).toBe(true)
        expect(w.text()).not.toContain('No radio stations')
    })
})
