import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

const stations = ref<Array<{ id: string; name: string; streamUrl: string }>>([])

vi.mock('@/composables/useSubsonicQueries', () => ({
    useRadioStations: () => ({ data: stations, isLoading: ref(false) }),
    useCreateRadioStation: () => ({ isPending: ref(false), mutate: vi.fn() }),
    useUpdateRadioStation: () => ({ isPending: ref(false), mutate: vi.fn() }),
    useDeleteRadioStation: () => ({ isPending: ref(false), mutate: vi.fn() })
}))

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ playNow: vi.fn() })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { getCoverArtUrl: (id: string) => `/cover/${id}` }
}))

vi.mock('primevue/useconfirm', () => ({
    useConfirm: () => ({ require: vi.fn() })
}))

// Stub the scaffold so the test asserts what RadioView passes to it (its own
// responsibility) rather than the scaffold's internal markup — the scaffold has
// its own spec. Slots are rendered so header actions stay clickable.
const ScaffoldStub = {
    name: 'ContentScaffold',
    props: ['title', 'summary'],
    template: '<div><slot name="actions" /><slot /></div>'
}
const DialogStub = {
    name: 'RadioStationDialog',
    props: ['visible', 'station', 'submitting'],
    template: '<div class="dialog-stub" />'
}

import RadioView from '@/views/RadioView.vue'

const mountView = () =>
    mount(RadioView, {
        global: {
            plugins: [PrimeVue],
            stubs: {
                ContentScaffold: ScaffoldStub,
                RadioStationDialog: DialogStub,
                ConfirmDialog: true
            }
        }
    })

beforeEach(() => {
    stations.value = []
})

describe('RadioView', () => {
    it('passes the Radio title and pluralized station count to the scaffold', () => {
        stations.value = [
            { id: '1', name: 'A', streamUrl: 'http://a' },
            { id: '2', name: 'B', streamUrl: 'http://b' }
        ]
        const scaffold = mountView().findComponent(ScaffoldStub)
        expect(scaffold.props('title')).toBe('Radio')
        expect(scaffold.props('summary')).toBe('2 stations')
    })

    it('uses the singular unit for a single station', () => {
        stations.value = [{ id: '1', name: 'A', streamUrl: 'http://a' }]
        expect(mountView().findComponent(ScaffoldStub).props('summary')).toBe('1 station')
    })

    it('passes an empty summary and shows the empty state when there are no stations', () => {
        const w = mountView()
        expect(w.findComponent(ScaffoldStub).props('summary')).toBe('')
        expect(w.find('.empty-state').exists()).toBe(true)
    })

    it('opens the create dialog from the header Add button', async () => {
        const w = mountView()
        expect(w.findComponent(DialogStub).props('visible')).toBe(false)
        const addBtn = w
            .findAll('button')
            .find((b) => b.text().includes('Add Station'))
        expect(addBtn).toBeTruthy()
        await addBtn!.trigger('click')
        expect(w.findComponent(DialogStub).props('visible')).toBe(true)
    })
})
