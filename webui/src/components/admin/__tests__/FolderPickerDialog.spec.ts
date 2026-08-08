import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { BrowseFolder } from '@/types/libraries'

const browseFoldersMock = vi.fn()
vi.mock('@/lib/api/Libraries', () => ({
    browseFolders: (...args: unknown[]) => browseFoldersMock(...args)
}))

import FolderPickerDialog from '@/components/admin/FolderPickerDialog.vue'

function folder(name: string, extra: Partial<BrowseFolder> = {}): BrowseFolder {
    return {
        name,
        path: `/${name}`,
        has_subfolders: false,
        is_symlink: false,
        ...extra
    }
}

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
            folders: [folder('srv', { has_subfolders: true })]
        })
        const w = mountPicker()
        await flushPromises()
        expect(browseFoldersMock).toHaveBeenCalledWith('/', false)
        expect(w.text()).toContain('srv')
    })

    it('re-fetches with show_hidden when the checkbox is ticked', async () => {
        browseFoldersMock.mockResolvedValueOnce({
            path: '/',
            folders: [folder('srv')]
        })
        browseFoldersMock.mockResolvedValueOnce({
            path: '/',
            folders: [folder('.mnt'), folder('srv')]
        })
        const w = mountPicker()
        await flushPromises()
        expect(w.text()).not.toContain('.mnt')

        await w.find('[data-testid="folder-picker-show-hidden"] input').setValue(true)
        await flushPromises()
        expect(browseFoldersMock).toHaveBeenLastCalledWith('/', true)
        expect(w.text()).toContain('.mnt')
    })

    it('clears a hidden selection when hidden folders are switched back off', async () => {
        browseFoldersMock.mockResolvedValue({
            path: '/',
            folders: [folder('.mnt')]
        })
        const w = mountPicker()
        await flushPromises()
        await w.find('[data-testid="folder-picker-show-hidden"] input').setValue(true)
        await flushPromises()
        await w.find('.p-tree-node-content').trigger('click')
        expect(w.text()).toContain('/.mnt')

        await w.find('[data-testid="folder-picker-show-hidden"] input').setValue(false)
        await flushPromises()
        expect(w.text()).toContain('No folder selected')
        expect(
            w.find('[data-testid="folder-picker-select"]').attributes('disabled')
        ).toBeDefined()
    })

    it('marks symlinked folders with the link icon, plain folders without', async () => {
        browseFoldersMock.mockResolvedValue({
            path: '/',
            folders: [folder('linked', { is_symlink: true }), folder('real')]
        })
        const w = mountPicker()
        await flushPromises()
        const rows = w.findAll('.p-tree-node')
        expect(rows).toHaveLength(2)
        expect(rows[0].find('.p-tree-node-icon').classes()).toContain('pi-link')
        expect(rows[1].find('.p-tree-node-icon').classes()).toContain('pi-folder')
    })

    it('expands a symlinked folder and selects a path under it', async () => {
        browseFoldersMock.mockImplementation((path: string) =>
            Promise.resolve(
                path === '/'
                    ? {
                          path: '/',
                          folders: [
                              folder('linked', { is_symlink: true, has_subfolders: true })
                          ]
                      }
                    : {
                          path,
                          folders: [folder('child', { path: '/linked/child' })]
                      }
            )
        )
        const w = mountPicker()
        await flushPromises()
        await w.find('.p-tree-node-toggle-button').trigger('click')
        await flushPromises()
        expect(browseFoldersMock).toHaveBeenLastCalledWith('/linked', false)
        expect(w.text()).toContain('child')

        await w.findAll('.p-tree-node-content')[1].trigger('click')
        await w.find('[data-testid="folder-picker-select"]').trigger('click')
        expect(w.emitted('select')).toEqual([['/linked/child']])
    })

    it('emits select with the chosen path on confirm', async () => {
        browseFoldersMock.mockResolvedValue({
            path: '/',
            folders: [folder('srv')]
        })
        const w = mountPicker()
        await flushPromises()
        await w.find('.p-tree-node-content').trigger('click')
        await w.find('[data-testid="folder-picker-select"]').trigger('click')
        expect(w.emitted('select')).toEqual([['/srv']])
        expect(w.emitted('update:visible')).toEqual([[false]])
    })
})
