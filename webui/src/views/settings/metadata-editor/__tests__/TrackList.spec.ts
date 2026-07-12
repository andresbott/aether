import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import TrackList from '@/views/settings/metadata-editor/TrackList.vue'
import type { Track } from '@/types/metadata'

// Stub the PrimeVue DataTable so we can drive its row-click / update:selection
// events (TrackList owns the selection logic) without rendering the real table.
const DataTableStub = {
    name: 'DataTable',
    props: ['value', 'selection', 'dataKey', 'rowClass'],
    emits: ['row-click', 'update:selection'],
    template: '<div><slot /></div>'
}
const stubs = {
    DataTable: DataTableStub,
    Column: { template: '<div><slot /></div>' },
    Button: { template: '<button />' }
}

const mkTrack = (over: Partial<Track> = {}): Track => ({
    path: 'a.mp3', name: 'a.mp3', title: '', artists: [], album_artists: [],
    album: '', year: 0, disc_number: 0, disc_subtitle: '', compilation: false,
    mb_artist_ids: [], mb_album_artist_ids: [], mb_release_id: '', mb_release_group_id: '',
    ...over
})

const tracks = Array.from({ length: 6 }, (_, i) => mkTrack({ path: `p${i}.mp3`, name: `p${i}` }))

function mountList(list: Track[] = tracks) {
    return mount(TrackList, {
        props: { tracks: list, isLoading: false, selection: [] },
        global: { stubs }
    })
}

// Reflect the last committed selection back onto the prop, like the parent view.
async function sync(wrapper: any) {
    const events = wrapper.emitted('update:selection')
    if (events?.length) await wrapper.setProps({ selection: events[events.length - 1][0] })
}

async function click(wrapper: any, index: number, mods: Partial<MouseEvent> = {}) {
    wrapper.findComponent(DataTableStub).vm.$emit('row-click', {
        originalEvent: { ctrlKey: false, metaKey: false, shiftKey: false, ...mods },
        data: wrapper.props('tracks')[index],
        index
    })
    await sync(wrapper)
}

const paths = (wrapper: any): string[] => {
    const events = wrapper.emitted('update:selection')
    return (events[events.length - 1][0] as Track[]).map((t) => t.path)
}

describe('TrackList selection', () => {
    it('plain click selects only the clicked row', async () => {
        const w = mountList()
        await click(w, 2)
        expect(paths(w)).toEqual(['p2.mp3'])
        await click(w, 4)
        expect(paths(w)).toEqual(['p4.mp3'])
    })

    it('Ctrl click toggles rows additively', async () => {
        const w = mountList()
        await click(w, 2, { ctrlKey: true })
        await click(w, 4, { ctrlKey: true })
        expect(paths(w)).toEqual(['p2.mp3', 'p4.mp3'])
        await click(w, 2, { ctrlKey: true })
        expect(paths(w)).toEqual(['p4.mp3'])
    })

    it('Shift click extends the range without dropping earlier selections', async () => {
        const w = mountList()
        await click(w, 1) // select the 2nd row
        await click(w, 3, { ctrlKey: true }) // add the 4th row (anchor)
        await click(w, 5, { shiftKey: true }) // shift to the 6th row
        // The earlier plain-selected row survives, plus the anchor→target range.
        expect(paths(w)).toEqual(['p1.mp3', 'p3.mp3', 'p4.mp3', 'p5.mp3'])

        // Re-dragging the range off the same pivot shrinks it back to the base.
        await click(w, 4, { shiftKey: true })
        expect(paths(w)).toEqual(['p1.mp3', 'p3.mp3', 'p4.mp3'])
    })

    it('filters error rows out of a checkbox-driven selection', async () => {
        const good = mkTrack({ path: 'good.mp3' })
        const bad = mkTrack({ path: 'bad.mp3', error: 'read error' })
        const w = mountList([good, bad])
        w.findComponent(DataTableStub).vm.$emit('update:selection', [good, bad])
        await w.vm.$nextTick()
        expect(w.emitted('update:selection')?.[0]?.[0]).toEqual([good])
    })
})
