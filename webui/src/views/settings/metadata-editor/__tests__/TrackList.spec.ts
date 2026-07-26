import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
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
    album: '', genres: [], year: 0, track_number: 0, disc_number: 0, disc_subtitle: '', compilation: false,
    mb_artist_ids: [], mb_album_artist_ids: [], mb_recording_id: '',
    mb_release_id: '', mb_release_group_id: '',
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

// A checkbox toggle: PrimeVue swallows the click (it lands inside .p-checkbox) and
// only re-emits update:selection with the additively toggled list, so the wrapper
// mousedown is the component's only view of the modifier keys.
async function checkboxToggle(wrapper: any, index: number, mods: Partial<MouseEvent> = {}) {
    await wrapper.find('.table-wrapper').trigger('mousedown', { shiftKey: false, ...mods })
    const track = wrapper.props('tracks')[index]
    const current: Track[] = wrapper.props('selection')
    const next = current.some((t) => t.path === track.path)
        ? current.filter((t) => t.path !== track.path)
        : [...current, track]
    wrapper.findComponent(DataTableStub).vm.$emit('update:selection', next)
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

    it('Shift checkbox toggle extends the range, like a Shift row click', async () => {
        const w = mountList()
        await checkboxToggle(w, 1) // check the 2nd row (anchor)
        await checkboxToggle(w, 4, { shiftKey: true }) // shift-check the 5th row
        expect(paths(w)).toEqual(['p1.mp3', 'p2.mp3', 'p3.mp3', 'p4.mp3'])

        // Same pivot, so re-dragging shrinks the range instead of accumulating.
        await checkboxToggle(w, 2, { shiftKey: true })
        expect(paths(w)).toEqual(['p1.mp3', 'p2.mp3'])
    })

    it('Shift checkbox toggle unions onto the earlier committed selection', async () => {
        const w = mountList()
        await click(w, 0) // plain-select the 1st row
        await checkboxToggle(w, 2) // check the 3rd row (new anchor + base)
        await checkboxToggle(w, 4, { shiftKey: true })
        expect(paths(w)).toEqual(['p0.mp3', 'p2.mp3', 'p3.mp3', 'p4.mp3'])
    })

    it('Shift row click extends from a checkbox-set anchor', async () => {
        const w = mountList()
        await checkboxToggle(w, 1)
        await click(w, 3, { shiftKey: true })
        expect(paths(w)).toEqual(['p1.mp3', 'p2.mp3', 'p3.mp3'])
    })

    it('a bare checkbox toggle stays additive', async () => {
        const w = mountList()
        await checkboxToggle(w, 1)
        await checkboxToggle(w, 4)
        expect(paths(w)).toEqual(['p1.mp3', 'p4.mp3'])
        await checkboxToggle(w, 1)
        expect(paths(w)).toEqual(['p4.mp3'])
    })

    it('Shift select-all is not treated as a range', async () => {
        const w = mountList()
        await checkboxToggle(w, 1)
        await w.find('.table-wrapper').trigger('mousedown', { shiftKey: true })
        w.findComponent(DataTableStub).vm.$emit('update:selection', tracks)
        await sync(w)
        expect(paths(w)).toEqual(tracks.map((t) => t.path))
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

describe('TrackList arrow-key navigation', () => {
    async function pressKey(wrapper: any, key: string) {
        await wrapper.find('.table-wrapper').trigger('keydown', { key })
        await sync(wrapper)
    }

    function mountWithSelection(list: Track[], selection: Track[]) {
        return mount(TrackList, {
            props: { tracks: list, isLoading: false, selection },
            global: { stubs }
        })
    }

    it('moves a single selection down and up', async () => {
        const w = mountWithSelection(tracks, [tracks[2]])
        await pressKey(w, 'ArrowDown')
        expect(paths(w)).toEqual(['p3.mp3'])
        await w.setProps({ selection: [tracks[3]] })
        await pressKey(w, 'ArrowUp')
        expect(paths(w)).toEqual(['p2.mp3'])
    })

    it('skips error rows when moving', async () => {
        const list = [
            mkTrack({ path: 'a.mp3' }),
            mkTrack({ path: 'bad.mp3', error: 'read error' }),
            mkTrack({ path: 'c.mp3' })
        ]
        const w = mountWithSelection(list, [list[0]])
        await pressKey(w, 'ArrowDown')
        expect(paths(w)).toEqual(['c.mp3'])
    })

    it('stays put at the list edges', async () => {
        const w = mountWithSelection(tracks, [tracks[0]])
        await w.find('.table-wrapper').trigger('keydown', { key: 'ArrowUp' })
        expect(w.emitted('update:selection')).toBeUndefined()
    })

    it('does nothing with a multi-selection', async () => {
        const w = mountWithSelection(tracks, [tracks[0], tracks[1]])
        await w.find('.table-wrapper').trigger('keydown', { key: 'ArrowDown' })
        expect(w.emitted('update:selection')).toBeUndefined()
    })
})

describe('TrackList staged marker', () => {
    // Mount the real PrimeVue DataTable so the marker column's body slot
    // renders per row.
    function mountReal(list: Track[], stagedPaths: ReadonlySet<string>) {
        return mount(TrackList, {
            props: { tracks: list, isLoading: false, selection: [], stagedPaths },
            global: {
                plugins: [PrimeVue],
                directives: { tooltip: {} }
            }
        })
    }

    it('marks only rows whose path is staged', () => {
        const w = mountReal(
            [mkTrack({ path: 'dirty.mp3' }), mkTrack({ path: 'clean.mp3' })],
            new Set(['dirty.mp3'])
        )
        const markers = w.findAll('[data-test="staged-marker"]')
        expect(markers).toHaveLength(1)
        const rows = w.findAll('tbody tr')
        expect(rows[0].find('[data-test="staged-marker"]').exists()).toBe(true)
        expect(rows[1].find('[data-test="staged-marker"]').exists()).toBe(false)
    })

    it('renders no marker without staged paths', () => {
        const w = mountReal([mkTrack({ path: 'clean.mp3' })], new Set<string>())
        expect(w.find('[data-test="staged-marker"]').exists()).toBe(false)
    })
})
