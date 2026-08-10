import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { Library } from '@/types/libraries'

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))
vi.mock('primevue/useconfirm', () => ({ useConfirm: () => ({ require: vi.fn() }) }))

// The panel destructures these, so the mocks must hand back real refs —
// a plain { value } object does not unwrap in the template.
const libraries = vi.hoisted(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return { current: [] as any[] }
})

vi.mock('@/composables/useLibraries', async () => {
    const { ref: vueRef } = await import('vue')
    return {
        useLibraries: () => ({ data: vueRef(libraries.current), isLoading: vueRef(false) }),
        useCreateLibrary: () => ({ mutate: vi.fn(), isPending: vueRef(false) }),
        useUpdateLibrary: () => ({ mutate: vi.fn(), isPending: vueRef(false) }),
        useDeleteLibrary: () => ({ mutate: vi.fn(), isPending: vueRef(false) })
    }
})

import LibrariesPanel from '@/components/admin/LibrariesPanel.vue'

function library(over: Partial<Library>): Library {
    return {
        id: 1,
        name: 'Main',
        path: '/srv/music',
        exclude_patterns: [],
        follow_symlinks: true,
        show_artists: true,
        default_view: 'albums',
        icon: 'folder',
        cover_style: 'auto',
        source: 'db',
        last_scan_started_at: null,
        created_at: '',
        updated_at: '',
        track_count: 0,
        ...over
    }
}

const mountPanel = (libs: Library[]) => {
    libraries.current = libs
    return mount(LibrariesPanel, {
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { teleport: true, ConfirmDialog: true, LibraryDialog: true }
        }
    })
}

describe('LibrariesPanel config-provisioned libraries', () => {
    it('lists libraries from both sources', async () => {
        const w = mountPanel([
            library({ id: 1, name: 'Jazz', source: 'db' }),
            library({ id: 2, name: 'Rock', path: '/music/rock', source: 'config' })
        ])
        await flushPromises()
        expect(w.text()).toContain('Jazz')
        expect(w.text()).toContain('Rock')
    })

    it('badges a config-provisioned library and offers no edit or delete action', async () => {
        const w = mountPanel([library({ id: 2, name: 'Rock', source: 'config' })])
        await flushPromises()
        expect(w.text()).toContain('From config')
        expect(w.find('.pi-pencil').exists()).toBe(false)
        expect(w.find('.pi-trash').exists()).toBe(false)
    })

    it('keeps edit and delete actions for a UI-created library', async () => {
        const w = mountPanel([library({ id: 1, name: 'Jazz', source: 'db' })])
        await flushPromises()
        expect(w.text()).not.toContain('From config')
        expect(w.find('.pi-pencil').exists()).toBe(true)
        expect(w.find('.pi-trash').exists()).toBe(true)
    })
})
