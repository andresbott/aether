import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
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

// Stub the form so the test drives `change` directly; stub the scaffold to expose
// title/summary/actions; stub ConfirmDialog and Button.
const FormStub = {
    name: 'RadioStationForm',
    props: ['station', 'prefill'],
    template: '<div class="form-stub" />'
}
const ScaffoldStub = {
    name: 'ContentScaffold',
    props: ['title', 'summary'],
    template: '<div><slot name="actions" /><slot /></div>'
}
const stubs = {
    RadioStationForm: FormStub,
    ContentScaffold: ScaffoldStub,
    ConfirmDialog: { template: '<div />' },
    Button: {
        props: ['label', 'disabled'],
        inheritAttrs: false,
        template:
            '<button :class="$attrs.class" :disabled="disabled" @click="$emit(\'click\')">{{ label }}</button>'
    }
}

import RadioStationDetailView from '@/views/RadioStationDetailView.vue'

const station: InternetRadioStation = {
    id: 's1',
    name: 'Jazz FM',
    streamUrl: 'http://stream/jazz',
    homepageUrl: 'http://jazzfm.example',
    coverArt: 'ca1'
}

const emitChange = (w: ReturnType<typeof mount>, valid: boolean, dirty = true) =>
    w.findComponent(FormStub).vm.$emit('change', {
        input: { name: 'Jazz FM', streamUrl: 'http://stream/jazz' },
        valid,
        dirty
    })

const mountView = (props: Record<string, unknown>) =>
    mount(RadioStationDetailView, { props, global: { stubs, directives: { tooltip: {} } } })

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
})

describe('RadioStationDetailView', () => {
    it('create mode: titles "Add station" and has a disabled Create until valid', async () => {
        const w = mountView({ create: true })
        expect(w.findComponent(ScaffoldStub).props('title')).toBe('Add station')
        expect(w.find('.create-station').attributes('disabled')).toBeDefined()
        emitChange(w, true)
        await w.vm.$nextTick()
        expect(w.find('.create-station').attributes('disabled')).toBeUndefined()
    })

    it('create mode: Create calls the create mutation and returns to /radio', async () => {
        const w = mountView({ create: true })
        emitChange(w, true)
        await w.vm.$nextTick()
        await w.find('.create-station').trigger('click')
        expect(createMutate).toHaveBeenCalledWith(
            expect.objectContaining({ name: 'Jazz FM', streamUrl: 'http://stream/jazz' }),
            expect.anything()
        )
        expect(push).toHaveBeenCalledWith({ name: 'radio' })
    })

    it('create mode: seeds the form prefill from query params', () => {
        route.query = { name: 'RP', streamUrl: 'http://rp', homepage: 'http://rp.com' }
        const w = mountView({ create: true })
        expect(w.findComponent(FormStub).props('prefill')).toMatchObject({
            name: 'RP',
            streamUrl: 'http://rp',
            homepageUrl: 'http://rp.com'
        })
    })

    it('edit mode: shows the station name and resolves it from the list', () => {
        const w = mountView({ id: 's1' })
        expect(w.findComponent(ScaffoldStub).props('title')).toBe('Jazz FM')
        expect(w.findComponent(FormStub).props('station')).toMatchObject({ id: 's1' })
    })

    it('edit mode: Save calls the update mutation with the id', async () => {
        const w = mountView({ id: 's1' })
        emitChange(w, true)
        await w.vm.$nextTick()
        await w.find('.save-station').trigger('click')
        expect(updateMutate).toHaveBeenCalledWith(
            expect.objectContaining({ id: 's1', name: 'Jazz FM' }),
            expect.anything()
        )
    })

    it('edit mode: Delete (confirmed) removes the station and returns to /radio', async () => {
        const w = mountView({ id: 's1' })
        await w.find('.delete-station').trigger('click')
        expect(deleteMutate).toHaveBeenCalledWith('s1', expect.anything())
        expect(push).toHaveBeenCalledWith({ name: 'radio' })
    })

    it('edit mode: Play enqueues the station', async () => {
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
