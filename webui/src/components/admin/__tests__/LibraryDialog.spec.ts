import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

import LibraryDialog from '@/components/admin/LibraryDialog.vue'
import type { Library, LibraryInput } from '@/types/libraries'

const baseLibrary: Library = {
    id: 1,
    name: 'Main',
    path: '/srv/music',
    exclude_patterns: [],
    follow_symlinks: true,
    show_artists: true,
    default_view: 'albums',
    icon: 'folder',
    cover_style: 'bauhaus',
    last_scan_started_at: null,
    created_at: '',
    updated_at: '',
    track_count: 0
}

const mountDialog = (library: Library | null) =>
    mount(LibraryDialog, {
        props: { visible: true, library, submitting: false },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { teleport: true }
        }
    })

describe('LibraryDialog cover style', () => {
    it('defaults cover_style to auto in create mode', async () => {
        const w = mountDialog(null)
        await flushPromises()
        const createBtn = w.findAll('button').find((b) => b.text().includes('Create'))!
        await createBtn.trigger('click')
        await flushPromises()
        const input = w.emitted('submit')![0][0] as LibraryInput
        expect(input.cover_style).toBe('auto')
    })

    it('submits the library cover_style unchanged in edit mode', async () => {
        const w = mountDialog(baseLibrary)
        await flushPromises()
        const saveBtn = w.findAll('button').find((b) => b.text().includes('Save'))!
        await saveBtn.trigger('click')
        await flushPromises()
        const input = w.emitted('submit')![0][0] as LibraryInput
        expect(input.cover_style).toBe('bauhaus')
    })

    it('offers auto plus all six styles in the dropdown', async () => {
        const w = mountDialog(null)
        await flushPromises()
        const dropdowns = w.findAllComponents({ name: 'Dropdown' })
        const styleDropdown = dropdowns.find((d) =>
            (d.props('options') as { value: string }[]).some((o) => o.value === 'auto')
        )!
        expect(styleDropdown).toBeTruthy()
        const values = (styleDropdown.props('options') as { value: string }[]).map((o) => o.value)
        expect(values).toEqual(['auto', 'classic', 'bauhaus', 'rings', 'waves', 'poster', 'remix'])
    })
})
