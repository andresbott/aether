import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'
import type { InternetRadioStation } from '@/types/subsonic'

const stations = ref<InternetRadioStation[]>([])
const isLoading = ref(false)
const createMutate = vi.fn((_i: unknown, o?: { onSuccess?: () => void }) => o?.onSuccess?.())
const updateMutate = vi.fn((_i: unknown, o?: { onSuccess?: () => void }) => o?.onSuccess?.())
const deleteMutate = vi.fn((_i: unknown, o?: { onSuccess?: () => void }) => o?.onSuccess?.())
vi.mock('@/composables/useSubsonicQueries', () => ({
    useRadioStations: () => ({ data: stations, isLoading }),
    useCreateRadioStation: () => ({ mutate: createMutate, isPending: ref(false) }),
    useUpdateRadioStation: () => ({ mutate: updateMutate, isPending: ref(false) }),
    useDeleteRadioStation: () => ({ mutate: deleteMutate, isPending: ref(false) })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        getCoverArtUrl: (id: string, size?: number) => `/cover/${id}?size=${size}`
    }
}))

const playNow = vi.fn()
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playNow }) }))
vi.mock('@/utils/radioSong', () => ({ stationToSong: (s: InternetRadioStation) => ({ id: s.id }) }))

const { fetchRadioFavicon } = vi.hoisted(() => ({ fetchRadioFavicon: vi.fn(async () => null) }))
vi.mock('@/lib/api/RadioBrowser', () => ({ fetchRadioFavicon }))

// Auto-accept the delete confirmation.
vi.mock('primevue/useconfirm', () => ({
    useConfirm: () => ({ require: (opts: { accept: () => void }) => opts.accept() })
}))

const push = vi.fn()
const back = vi.fn()
const route = { query: {} as Record<string, string> }
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ push, back }),
    onBeforeRouteLeave: vi.fn()
}))

const ScaffoldStub = {
    name: 'ContentScaffold',
    props: ['title', 'summary'],
    template: '<div><slot name="actions" /><slot /></div>'
}
const stubs = {
    ContentScaffold: ScaffoldStub,
    ConfirmDialog: { template: '<div />' }
}

import RadioStationDetailView from '@/views/RadioStationDetailView.vue'

const station: InternetRadioStation = {
    id: 's1',
    name: 'Jazz FM',
    streamUrl: 'http://stream/jazz',
    homepageUrl: 'http://jazzfm.example',
    coverArt: 'ca1'
}

const mountView = (props: Record<string, unknown>) =>
    mount(RadioStationDetailView, {
        props,
        global: { plugins: [PrimeVue], stubs, directives: { tooltip: {} } }
    })

// name / streamUrl / homepage inputs in the hero's edit column (the cover picker
// lives in the flip back face, not in .edit-only).
const editInputs = (w: ReturnType<typeof mountView>) => w.findAll('.edit-only input')

beforeEach(() => {
    stations.value = [station]
    isLoading.value = false
    route.query = {}
    createMutate.mockClear()
    updateMutate.mockClear()
    deleteMutate.mockClear()
    playNow.mockClear()
    push.mockClear()
    fetchRadioFavicon.mockClear()
    global.URL.createObjectURL = vi.fn(() => 'blob:mock')
    global.URL.revokeObjectURL = vi.fn()
})

describe('RadioStationDetailView', () => {
    it('create mode: has a disabled Save until valid', async () => {
        const w = mountView({ create: true })
        expect(w.findComponent(ScaffoldStub).props('title')).toBe('Add station')
        expect(w.find('.edit-action-save').attributes('disabled')).toBeDefined()

        const inputs = editInputs(w)
        await inputs[0].setValue('Jazz FM')
        await inputs[1].setValue('http://stream/jazz')
        expect(w.find('.edit-action-save').attributes('disabled')).toBeUndefined()
    })

    it('create mode: Save calls the create mutation and returns to /radio', async () => {
        const w = mountView({ create: true })
        const inputs = editInputs(w)
        await inputs[0].setValue('Jazz FM')
        await inputs[1].setValue('http://stream/jazz')
        await w.find('.edit-action-save').trigger('click')
        expect(createMutate).toHaveBeenCalledWith(
            expect.objectContaining({ name: 'Jazz FM', streamUrl: 'http://stream/jazz' }),
            expect.anything()
        )
        expect(push).toHaveBeenCalledWith({ name: 'radio' })
    })

    it('create mode: has no Delete button', () => {
        const w = mountView({ create: true })
        expect(w.find('.edit-action-delete').exists()).toBe(false)
    })

    it('create mode: seeds the form from query params', () => {
        route.query = { name: 'RP', streamUrl: 'http://rp', homepage: 'http://rp.com' }
        const w = mountView({ create: true })
        const inputs = editInputs(w)
        expect((inputs[0].element as HTMLInputElement).value).toBe('RP')
        expect((inputs[1].element as HTMLInputElement).value).toBe('http://rp')
        expect((inputs[2].element as HTMLInputElement).value).toBe('http://rp.com')
    })

    it('existing station opens read-only with Play + pencil', () => {
        const w = mountView({ id: 's1' })
        expect(w.findComponent(ScaffoldStub).props('title')).toBe('')
        expect(w.find('.hero-name').text()).toBe('Jazz FM')
        expect(w.find('.play-station').exists()).toBe(true)
        expect(w.find('.edit-action-edit').exists()).toBe(true)
        expect(w.find('.edit-action-save').exists()).toBe(false)
    })

    it('edit mode: Save calls the update mutation with the id', async () => {
        const w = mountView({ id: 's1' })
        await w.find('.edit-action-edit').trigger('click')
        await w.find('.edit-action-save').trigger('click')
        expect(updateMutate).toHaveBeenCalledWith(
            expect.objectContaining({ id: 's1', name: 'Jazz FM' }),
            expect.anything()
        )
    })

    it('edit mode: Cancel reverts in-progress edits and exits edit mode', async () => {
        const w = mountView({ id: 's1' })
        await w.find('.edit-action-edit').trigger('click')
        await editInputs(w)[0].setValue('Changed Name')
        await w.find('.edit-action-cancel').trigger('click')
        expect(updateMutate).not.toHaveBeenCalled()
        expect(w.find('.edit-action-save').exists()).toBe(false)
        expect(w.find('.edit-action-edit').exists()).toBe(true)
        // Re-entering edit mode shows the reverted value.
        await w.find('.edit-action-edit').trigger('click')
        expect((editInputs(w)[0].element as HTMLInputElement).value).toBe('Jazz FM')
    })

    it('edit mode: Delete (confirmed) removes the station and returns to /radio', async () => {
        const w = mountView({ id: 's1' })
        await w.find('.edit-action-edit').trigger('click')
        await w.find('.edit-action-delete').trigger('click')
        expect(deleteMutate).toHaveBeenCalledWith('s1', expect.anything())
        expect(push).toHaveBeenCalledWith({ name: 'radio' })
    })

    it('read mode: Play enqueues the station', async () => {
        const w = mountView({ id: 's1' })
        await w.find('.play-station').trigger('click')
        expect(playNow).toHaveBeenCalledWith({ id: 's1' })
    })

    it('edit mode: shows not-found when the id is absent after loading', () => {
        stations.value = []
        isLoading.value = false
        const w = mountView({ id: 'missing' })
        expect(w.text()).toContain('not found')
    })
})
