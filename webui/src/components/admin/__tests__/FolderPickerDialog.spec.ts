import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

const browseFoldersMock = vi.fn()
vi.mock('@/lib/api/Libraries', () => ({
    browseFolders: (...args: unknown[]) => browseFoldersMock(...args)
}))

import FolderPickerDialog from '@/components/admin/FolderPickerDialog.vue'

function mountPicker() {
    return mount(FolderPickerDialog, {
        props: { visible: true },
        global: {
            plugins: [PrimeVue],
            stubs: { teleport: true }
        }
    })
}

beforeEach(() => {
    browseFoldersMock.mockReset()
})

describe('FolderPickerDialog', () => {
    it('loads the filesystem root when opened', async () => {
        browseFoldersMock.mockResolvedValue({
            path: '/',
            folders: [{ name: 'srv', path: '/srv', has_subfolders: true }]
        })
        const w = mountPicker()
        await flushPromises()
        expect(browseFoldersMock).toHaveBeenCalledWith('/')
        expect(w.text()).toContain('srv')
    })

    it('emits select with the chosen path on confirm', async () => {
        browseFoldersMock.mockResolvedValue({
            path: '/',
            folders: [{ name: 'srv', path: '/srv', has_subfolders: false }]
        })
        const w = mountPicker()
        await flushPromises()
        await w.find('.p-tree-node-content').trigger('click')
        await w.find('[data-testid="folder-picker-select"]').trigger('click')
        expect(w.emitted('select')).toEqual([['/srv']])
        expect(w.emitted('update:visible')).toEqual([[false]])
    })
})
