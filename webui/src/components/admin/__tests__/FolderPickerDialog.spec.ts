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
    it('loads the scan-folder roots when opened, one node per root labelled with its name', async () => {
        browseFoldersMock.mockResolvedValue({
            path: '',
            folders: [
                folder('Music', { path: '/mnt/music', has_subfolders: true }),
                folder('Podcasts', { path: '/data/podcasts', has_subfolders: true })
            ]
        })
        const w = mountPicker()
        await flushPromises()
        expect(browseFoldersMock).toHaveBeenCalledWith(undefined, false)
        expect(browseFoldersMock).not.toHaveBeenCalledWith('/', expect.anything())
        const rows = w.findAll('.p-tree-node')
        expect(rows).toHaveLength(2)
        expect(rows[0].text()).toContain('Music')
        expect(rows[1].text()).toContain('Podcasts')
    })

    it('marks the roots with the database icon, and symlinked folders below them with the link icon', async () => {
        browseFoldersMock.mockImplementation((path?: string) =>
            path === undefined
                ? Promise.resolve({
                      path: '',
                      folders: [folder('Music', { path: '/mnt/music', has_subfolders: true })]
                  })
                : Promise.resolve({
                      path,
                      folders: [
                          folder('linked', { path: '/mnt/music/linked', is_symlink: true }),
                          folder('real', { path: '/mnt/music/real' })
                      ]
                  })
        )
        const w = mountPicker()
        await flushPromises()
        await w.find('.p-tree-node-toggle-button').trigger('click')
        await flushPromises()
        const rows = w.findAll('.p-tree-node')
        expect(rows).toHaveLength(3)
        expect(rows[0].find('.p-tree-node-icon').classes()).toContain('ms-database')
        expect(rows[1].find('.p-tree-node-icon').classes()).toContain('ms-link')
        expect(rows[2].find('.p-tree-node-icon').classes()).toContain('ms-folder')
    })

    it("expanding a root asks the server for that root's own path", async () => {
        browseFoldersMock.mockImplementation((path?: string) =>
            path === undefined
                ? Promise.resolve({
                      path: '',
                      folders: [folder('Music', { path: '/mnt/music', has_subfolders: true })]
                  })
                : Promise.resolve({ path, folders: [] })
        )
        const w = mountPicker()
        await flushPromises()
        await w.find('.p-tree-node-toggle-button').trigger('click')
        await flushPromises()
        expect(browseFoldersMock).toHaveBeenLastCalledWith('/mnt/music', false)
    })

    it('reloads from the roots and re-expands open branches when "Show hidden folders" is toggled', async () => {
        browseFoldersMock.mockImplementation((path?: string, hidden?: boolean) => {
            if (path === undefined) {
                return Promise.resolve({
                    path: '',
                    folders: [folder('Music', { path: '/mnt/music', has_subfolders: true })]
                })
            }
            if (path === '/mnt/music') {
                return Promise.resolve({
                    path,
                    folders: hidden
                        ? [
                              folder('.hidden', { path: '/mnt/music/.hidden' }),
                              folder('Artist', { path: '/mnt/music/Artist' })
                          ]
                        : [folder('Artist', { path: '/mnt/music/Artist' })]
                })
            }
            return Promise.resolve({ path, folders: [] })
        })
        const w = mountPicker()
        await flushPromises()
        expect(browseFoldersMock).toHaveBeenCalledWith(undefined, false)

        await w.find('.p-tree-node-toggle-button').trigger('click')
        await flushPromises()
        expect(browseFoldersMock).toHaveBeenLastCalledWith('/mnt/music', false)
        expect(w.text()).toContain('Artist')
        expect(w.text()).not.toContain('.hidden')

        await w.find('[data-testid="folder-picker-show-hidden"] input').setValue(true)
        await flushPromises()
        expect(browseFoldersMock).toHaveBeenCalledWith(undefined, true)
        expect(browseFoldersMock).toHaveBeenLastCalledWith('/mnt/music', true)
        expect(w.text()).toContain('.hidden')
        expect(w.text()).toContain('Artist')
    })

    // A selection is dropped because the rebuilt tree no longer CONTAINS it —
    // never because of how its path is spelled.
    it('clears a selection the reloaded tree no longer shows when hidden folders are switched off', async () => {
        browseFoldersMock.mockImplementation((_path?: string, hidden?: boolean) =>
            Promise.resolve({
                path: '',
                folders: hidden
                    ? [folder('.hidden', { path: '/mnt/.hidden' }), folder('Music', { path: '/mnt/music' })]
                    : [folder('Music', { path: '/mnt/music' })]
            })
        )
        const w = mountPicker()
        await flushPromises()
        await w.find('[data-testid="folder-picker-show-hidden"] input').setValue(true)
        await flushPromises()
        await w.findAll('.p-tree-node-content')[0].trigger('click')
        expect(w.text()).toContain('/mnt/.hidden')

        await w.find('[data-testid="folder-picker-show-hidden"] input').setValue(false)
        await flushPromises()
        expect(w.text()).toContain('No folder selected')
        expect(
            w.find('[data-testid="folder-picker-select"]').attributes('disabled')
        ).toBeDefined()
    })

    // A scan folder may itself live under a dot-directory — /home/x/.datos/music
    // is the shape this project's own owner runs — and everything below it is
    // perfectly visible, whatever "Show hidden folders" is set to.
    it('keeps a selection under a dot-directory root when hidden folders are toggled', async () => {
        browseFoldersMock.mockImplementation((path?: string) =>
            path === undefined
                ? Promise.resolve({
                      path: '',
                      folders: [folder('music', { path: '/home/x/.datos/music', has_subfolders: true })]
                  })
                : Promise.resolve({
                      path,
                      folders: [folder('Album', { path: '/home/x/.datos/music/Album' })]
                  })
        )
        const w = mountPicker()
        await flushPromises()
        await w.find('.p-tree-node-toggle-button').trigger('click')
        await flushPromises()
        await w.findAll('.p-tree-node-content')[1].trigger('click')
        expect(w.text()).toContain('/home/x/.datos/music/Album')

        await w.find('[data-testid="folder-picker-show-hidden"] input').setValue(true)
        await flushPromises()
        await w.find('[data-testid="folder-picker-show-hidden"] input').setValue(false)
        await flushPromises()

        expect(w.text()).toContain('/home/x/.datos/music/Album')
        expect(
            w.find('[data-testid="folder-picker-select"]').attributes('disabled')
        ).toBeUndefined()
    })

    it('shows the error banner when expanding an unmounted root, and leaves the tree usable', async () => {
        browseFoldersMock.mockImplementation((path?: string) => {
            if (path === undefined) {
                return Promise.resolve({
                    path: '',
                    folders: [
                        folder('Broken', { path: '/mnt/broken', has_subfolders: true }),
                        folder('Music', { path: '/mnt/music', has_subfolders: true })
                    ]
                })
            }
            if (path === '/mnt/broken') {
                return Promise.reject({
                    response: {
                        status: 400,
                        data: {
                            type: 'https://aether.local/probs/validation_error',
                            title: 'Bad Request',
                            status: 400,
                            detail: 'scan folder "Broken" is not mounted'
                        }
                    }
                })
            }
            return Promise.resolve({ path, folders: [] })
        })
        const w = mountPicker()
        await flushPromises()

        await w.findAll('.p-tree-node-toggle-button')[0].trigger('click')
        await flushPromises()
        expect(w.find('.error-banner').text()).toContain('scan folder "Broken" is not mounted')

        // The tree stays usable: the other root can still be selected and confirmed.
        await w.findAll('.p-tree-node-content')[1].trigger('click')
        expect(
            w.find('[data-testid="folder-picker-select"]').attributes('disabled')
        ).toBeUndefined()
        await w.find('[data-testid="folder-picker-select"]').trigger('click')
        expect(w.emitted('select')).toEqual([['/mnt/music']])
    })

    // Expanding an unmounted root is a designed flow, not a broken state: the
    // banner it raises must not outlive the next load that works.
    it('clears the error banner once another expand succeeds', async () => {
        browseFoldersMock.mockImplementation((path?: string) => {
            if (path === undefined) {
                return Promise.resolve({
                    path: '',
                    folders: [
                        folder('Broken', { path: '/mnt/broken', has_subfolders: true }),
                        folder('Music', { path: '/mnt/music', has_subfolders: true })
                    ]
                })
            }
            if (path === '/mnt/broken') {
                return Promise.reject({
                    response: {
                        status: 400,
                        data: {
                            type: 'https://aether.local/probs/validation_error',
                            title: 'Bad Request',
                            status: 400,
                            detail: 'scan folder "Broken" is not mounted'
                        }
                    }
                })
            }
            return Promise.resolve({
                path,
                folders: [folder('Artist', { path: '/mnt/music/Artist' })]
            })
        })
        const w = mountPicker()
        await flushPromises()

        await w.findAll('.p-tree-node-toggle-button')[0].trigger('click')
        await flushPromises()
        expect(w.find('.error-banner').exists()).toBe(true)

        await w.findAll('.p-tree-node-toggle-button')[1].trigger('click')
        await flushPromises()
        expect(w.find('.error-banner').exists()).toBe(false)
        expect(w.text()).toContain('Artist')
    })

    it('shows no symlink warning for a plain folder', async () => {
        browseFoldersMock.mockResolvedValue({
            path: '',
            folders: [folder('Music', { path: '/mnt/music', has_subfolders: true })]
        })
        const w = mountPicker()
        await flushPromises()
        await w.find('.p-tree-node-content').trigger('click')
        expect(w.find('[data-testid="folder-picker-symlink-warning"]').exists()).toBe(false)
        expect(
            w.find('[data-testid="folder-picker-select"]').attributes('disabled')
        ).toBeUndefined()
    })

    it('warns when the selected folder is itself a symlink, and still allows Select', async () => {
        browseFoldersMock.mockImplementation((path?: string) =>
            path === undefined
                ? Promise.resolve({
                      path: '',
                      folders: [folder('Music', { path: '/mnt/music', has_subfolders: true })]
                  })
                : Promise.resolve({
                      path,
                      folders: [folder('linked', { path: '/mnt/music/linked', is_symlink: true })]
                  })
        )
        const w = mountPicker()
        await flushPromises()
        await w.find('.p-tree-node-toggle-button').trigger('click')
        await flushPromises()
        await w.findAll('.p-tree-node-content')[1].trigger('click')

        const warning = w.find('[data-testid="folder-picker-symlink-warning"]')
        expect(warning.exists()).toBe(true)
        expect(warning.text()).toBe(
            'A library path filter on or below a symbolic link matches nothing today: tracks are recorded under the real location the link points to. Pick the real folder instead.'
        )
        expect(
            w.find('[data-testid="folder-picker-select"]').attributes('disabled')
        ).toBeUndefined()
        await w.find('[data-testid="folder-picker-select"]').trigger('click')
        expect(w.emitted('select')).toEqual([['/mnt/music/linked']])
    })

    it('warns when the selected folder lies below a symlink, even though it is not one itself', async () => {
        browseFoldersMock.mockImplementation((path?: string) => {
            if (path === undefined) {
                return Promise.resolve({
                    path: '',
                    folders: [folder('Music', { path: '/mnt/music', has_subfolders: true })]
                })
            }
            if (path === '/mnt/music') {
                return Promise.resolve({
                    path,
                    folders: [
                        folder('linked', {
                            path: '/mnt/music/linked',
                            is_symlink: true,
                            has_subfolders: true
                        })
                    ]
                })
            }
            return Promise.resolve({
                path,
                folders: [folder('inside', { path: '/mnt/music/linked/inside' })]
            })
        })
        const w = mountPicker()
        await flushPromises()
        await w.find('.p-tree-node-toggle-button').trigger('click')
        await flushPromises()
        await w.findAll('.p-tree-node-toggle-button')[1].trigger('click')
        await flushPromises()
        await w.findAll('.p-tree-node-content')[2].trigger('click')

        const warning = w.find('[data-testid="folder-picker-symlink-warning"]')
        expect(warning.exists()).toBe(true)
        expect(
            w.find('[data-testid="folder-picker-select"]').attributes('disabled')
        ).toBeUndefined()
        await w.find('[data-testid="folder-picker-select"]').trigger('click')
        expect(w.emitted('select')).toEqual([['/mnt/music/linked/inside']])
    })

    // The two features together: symlink ancestry is carried on the nodes, so a
    // reload that rebuilds every node must rebuild the ancestry with it.
    it('keeps warning about a selection below a symlink after the tree reloads', async () => {
        browseFoldersMock.mockImplementation((path?: string) => {
            if (path === undefined) {
                return Promise.resolve({
                    path: '',
                    folders: [folder('Music', { path: '/mnt/music', has_subfolders: true })]
                })
            }
            if (path === '/mnt/music') {
                return Promise.resolve({
                    path,
                    folders: [
                        folder('linked', {
                            path: '/mnt/music/linked',
                            is_symlink: true,
                            has_subfolders: true
                        })
                    ]
                })
            }
            return Promise.resolve({
                path,
                folders: [folder('inside', { path: '/mnt/music/linked/inside' })]
            })
        })
        const w = mountPicker()
        await flushPromises()
        await w.find('.p-tree-node-toggle-button').trigger('click')
        await flushPromises()
        await w.findAll('.p-tree-node-toggle-button')[1].trigger('click')
        await flushPromises()
        await w.findAll('.p-tree-node-content')[2].trigger('click')
        expect(w.find('[data-testid="folder-picker-symlink-warning"]').exists()).toBe(true)

        await w.find('[data-testid="folder-picker-show-hidden"] input').setValue(true)
        await flushPromises()

        expect(w.text()).toContain('/mnt/music/linked/inside')
        expect(w.find('[data-testid="folder-picker-symlink-warning"]').exists()).toBe(true)
    })

    it('emits select with the chosen path on confirm', async () => {
        browseFoldersMock.mockResolvedValue({
            path: '',
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
