import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import type { InternetRadioStation } from '@/types/subsonic'
import type { RadioBrowserStation } from '@/types/radiobrowser'

const stations = ref<InternetRadioStation[]>([])
const createMutate = vi.fn()
const updateMutate = vi.fn()
const deleteMutate = vi.fn()
const requireMock = vi.fn()
const fetchFaviconMock = vi.fn()

vi.mock('@/composables/useSubsonicQueries', () => ({
    useRadioStations: () => ({ data: stations, isLoading: ref(false) }),
    useCreateRadioStation: () => ({ isPending: ref(false), mutate: createMutate }),
    useUpdateRadioStation: () => ({ isPending: ref(false), mutate: updateMutate }),
    useDeleteRadioStation: () => ({ isPending: ref(false), mutate: deleteMutate })
}))

vi.mock('primevue/useconfirm', () => ({ useConfirm: () => ({ require: requireMock }) }))

vi.mock('@/lib/api/RadioBrowser', () => ({
    fetchRadioFavicon: (...args: any[]) => fetchFaviconMock(...args)
}))

import RadioStationsView from '@/views/settings/RadioStationsView.vue'

const ListStub = {
    name: 'StationList',
    props: ['stations', 'selectedId', 'isLoading'],
    emits: ['select'],
    template: '<div class="station-list-stub" />'
}
const PanelStub = {
    name: 'StationEditPanel',
    props: ['station', 'adding', 'submitting', 'initial'],
    emits: ['save', 'delete'],
    template: '<div class="edit-panel-stub" />'
}
const SearchDialogStub = {
    name: 'StationSearchDialog',
    props: ['visible'],
    emits: ['update:visible', 'select'],
    template: '<div class="search-dialog-stub" />'
}
const Passthrough = { template: '<div><slot /></div>' }
const ButtonStub = {
    props: ['label'],
    inheritAttrs: false,
    template: '<button @click="$emit(\'click\')">{{ label }}</button>'
}

const mountView = () =>
    mount(RadioStationsView, {
        global: {
            stubs: {
                StationList: ListStub,
                StationEditPanel: PanelStub,
                StationSearchDialog: SearchDialogStub,
                Splitter: Passthrough,
                SplitterPanel: Passthrough,
                Button: ButtonStub,
                ConfirmDialog: true
            }
        }
    })

const station: InternetRadioStation = {
    id: 'r-1',
    name: 'BBC Radio 1',
    streamUrl: 'http://bbc/stream'
}

const clickAdd = async (w: ReturnType<typeof mountView>) => {
    const btn = w.findAll('button').find((b) => b.text().includes('Add Station'))
    await btn!.trigger('click')
}

const clickSearch = async (w: ReturnType<typeof mountView>) => {
    const btn = w.findAll('button').find((b) => b.text().includes('Search Online'))
    await btn!.trigger('click')
}

const rbStation: RadioBrowserStation = {
    name: 'Radio Paradise',
    streamUrl: 'http://rp/stream',
    homepage: 'https://radioparadise.com',
    favicon: 'https://rp/favicon.png',
    tags: 'eclectic',
    country: 'United States',
    countryCode: 'US',
    language: 'english',
    codec: 'MP3',
    bitrate: 320,
    votes: 999,
    uuid: 'u1'
}

beforeEach(() => {
    stations.value = []
    createMutate.mockClear()
    updateMutate.mockClear()
    deleteMutate.mockClear()
    requireMock.mockClear()
    fetchFaviconMock.mockReset()
    fetchFaviconMock.mockResolvedValue(null)
})

describe('RadioStationsView', () => {
    it('shows a pluralized station count', () => {
        stations.value = [station, { id: 'r-2', name: 'Jazz', streamUrl: 'http://j' }]
        expect(mountView().text()).toContain('2 stations')
    })

    it('feeds the fetched stations to the list', () => {
        stations.value = [station]
        const list = mountView().findComponent(ListStub)
        expect(list.props('stations')).toEqual([station])
    })

    it('puts the panel in add mode when Add Station is clicked', async () => {
        const w = mountView()
        await clickAdd(w)
        const panel = w.findComponent(PanelStub)
        expect(panel.props('adding')).toBe(true)
        expect(panel.props('station')).toBe(null)
    })

    it('loads a selected station into the panel and highlights it in the list', async () => {
        stations.value = [station]
        const w = mountView()
        w.findComponent(ListStub).vm.$emit('select', station)
        await w.vm.$nextTick()
        expect(w.findComponent(PanelStub).props('station')).toEqual(station)
        expect(w.findComponent(PanelStub).props('adding')).toBe(false)
        expect(w.findComponent(ListStub).props('selectedId')).toBe('r-1')
    })

    it('creates a station when the panel saves in add mode', async () => {
        const w = mountView()
        await clickAdd(w)
        const input = { name: 'New', streamUrl: 'http://new' }
        w.findComponent(PanelStub).vm.$emit('save', input)
        expect(createMutate).toHaveBeenCalledTimes(1)
        expect(createMutate.mock.calls[0][0]).toEqual(input)
    })

    it('updates a station (with its id) when the panel saves in edit mode', async () => {
        stations.value = [station]
        const w = mountView()
        w.findComponent(ListStub).vm.$emit('select', station)
        await w.vm.$nextTick()
        w.findComponent(PanelStub).vm.$emit('save', {
            name: 'BBC R1',
            streamUrl: 'http://bbc/stream'
        })
        expect(updateMutate).toHaveBeenCalledTimes(1)
        expect(updateMutate.mock.calls[0][0]).toMatchObject({
            id: 'r-1',
            name: 'BBC R1',
            streamUrl: 'http://bbc/stream'
        })
        expect(createMutate).not.toHaveBeenCalled()
    })

    it('confirms before deleting, then deletes by id', async () => {
        stations.value = [station]
        const w = mountView()
        w.findComponent(ListStub).vm.$emit('select', station)
        await w.vm.$nextTick()
        w.findComponent(PanelStub).vm.$emit('delete')

        expect(requireMock).toHaveBeenCalledTimes(1)
        expect(deleteMutate).not.toHaveBeenCalled()
        // Run the confirm's accept callback to simulate the user confirming.
        requireMock.mock.calls[0][0].accept()
        expect(deleteMutate).toHaveBeenCalledTimes(1)
        expect(deleteMutate.mock.calls[0][0]).toBe('r-1')
    })

    it('opens the search dialog when Search Online is clicked', async () => {
        const w = mountView()
        expect(w.findComponent(SearchDialogStub).props('visible')).toBe(false)
        await clickSearch(w)
        expect(w.findComponent(SearchDialogStub).props('visible')).toBe(true)
    })

    it('imports a picked station into add mode with prefilled fields, then folds in the favicon cover', async () => {
        const file = new File(['x'], 'favicon.png', { type: 'image/png' })
        fetchFaviconMock.mockResolvedValue(file)
        const w = mountView()

        w.findComponent(SearchDialogStub).vm.$emit('select', rbStation)
        await w.vm.$nextTick()

        const panel = w.findComponent(PanelStub)
        expect(panel.props('adding')).toBe(true)
        expect(panel.props('station')).toBe(null)
        expect(panel.props('initial')).toMatchObject({
            name: 'Radio Paradise',
            streamUrl: 'http://rp/stream',
            homepageUrl: 'https://radioparadise.com'
        })
        // Picking closes the dialog.
        expect(w.findComponent(SearchDialogStub).props('visible')).toBe(false)

        // The background favicon fetch resolves and is folded into the prefill.
        await flushPromises()
        expect(fetchFaviconMock).toHaveBeenCalledWith('https://rp/favicon.png')
        expect(w.findComponent(PanelStub).props('initial').coverFile).toBe(file)
    })

    it('imports a station without a favicon without setting a cover', async () => {
        const w = mountView()
        w.findComponent(SearchDialogStub).vm.$emit('select', { ...rbStation, favicon: '' })
        await flushPromises()
        expect(fetchFaviconMock).not.toHaveBeenCalled()
        expect(w.findComponent(PanelStub).props('initial').coverFile).toBeUndefined()
    })
})
