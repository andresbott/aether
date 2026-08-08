import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

const route = { query: {} as Record<string, string> }
const replace = vi.fn()
const push = vi.fn()
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace, push })
}))

const stations = ref<Array<{ id: string; name: string; streamUrl: string }>>([])
vi.mock('@/composables/useSubsonicQueries', () => ({
    useRadioStations: () => ({ data: stations, isLoading: ref(false) })
}))

const isAdmin = ref(true)
vi.mock('@/composables/useAuth', () => ({ useAuth: () => ({ isAdmin }) }))

// Stub the scaffold so the test asserts what RadioView passes to it (its own
// responsibility) rather than the scaffold's internal markup — the scaffold has
// its own spec. Slots are rendered so header actions stay clickable.
const ScaffoldStub = {
    name: 'ContentScaffold',
    props: ['title', 'summary'],
    template: '<div><slot name="actions" /><slot /></div>'
}
const GridStub = { name: 'RadioStationGrid', template: '<div class="radio-grid-stub" />' }
const ListStub = { name: 'RadioStationListView', template: '<div class="radio-list-stub" />' }
import RadioView from '@/views/RadioView.vue'
import SelectButton from 'primevue/selectbutton'

const mountView = () =>
    mount(RadioView, {
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: {
                ContentScaffold: ScaffoldStub,
                RadioStationGrid: GridStub,
                RadioStationListView: ListStub,
                Button: {
                    props: ['label'],
                    inheritAttrs: false,
                    template:
                        '<button :class="$attrs.class" @click="$emit(\'click\')">{{ label }}</button>'
                }
            }
        }
    })

beforeEach(() => {
    replace.mockReset()
    push.mockReset()
    route.query = {}
    stations.value = []
    isAdmin.value = true
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

    it('passes an empty summary when there are no stations', () => {
        expect(mountView().findComponent(ScaffoldStub).props('summary')).toBe('')
    })

    it('renders the grid by default', () => {
        const w = mountView()
        expect(w.findComponent(GridStub).exists()).toBe(true)
        expect(w.findComponent(ListStub).exists()).toBe(false)
    })

    it('renders the list when the layout query is list', () => {
        route.query = { view: 'list' }
        const w = mountView()
        expect(w.findComponent(ListStub).exists()).toBe(true)
        expect(w.findComponent(GridStub).exists()).toBe(false)
    })

    it('toggling the layout updates the route query', async () => {
        const w = mountView()
        w.findComponent(SelectButton).vm.$emit('update:modelValue', 'list')
        await w.vm.$nextTick()
        expect(replace).toHaveBeenCalledWith({ query: { view: 'list' } })
    })

    it('renders an add icon button and no Discover button', () => {
        const w = mountView()
        expect(w.find('.add-station').exists()).toBe(true)
        expect(w.find('.discover-station').exists()).toBe(false)
    })

    it('the add button navigates to the create route', async () => {
        const w = mountView()
        await w.find('.add-station').trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'radio-station-new' })
    })

    it('hides the add button for non-admins (station CRUD is admin-only)', () => {
        isAdmin.value = false
        expect(mountView().find('.add-station').exists()).toBe(false)
    })
})
