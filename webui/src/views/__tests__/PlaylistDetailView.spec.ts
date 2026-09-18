import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'
import FileUpload from 'primevue/fileupload'

const { toastAdd } = vi.hoisted(() => ({ toastAdd: vi.fn() }))
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: toastAdd }) }))

const playlist = ref<any>(null)
const replaceIsPending = ref(false)
const coverIsPending = ref(false)
vi.mock('@/composables/useSubsonicQueries', () => ({
    usePlaylist: () => ({ data: playlist, isLoading: ref(false), error: ref(null) }),
    useUpdatePlaylist: () => ({ mutate: updateMutate, mutateAsync: updateAsync, isPending: ref(false) }),
    useUpdatePlaylistCover: () => ({
        mutate: coverMutate,
        mutateAsync: coverAsync,
        isPending: coverIsPending
    }),
    useDeletePlaylist: () => ({ mutate: deleteMutate }),
    useReplacePlaylistTracks: () => ({
        mutate: replaceMutate,
        mutateAsync: replaceAsync,
        isPending: replaceIsPending
    }),
    useTogglePlaylistStar: () => ({ mutate: vi.fn() }),
    // The track rows carry a favorite toggle of their own (per-song, not the
    // playlist's), whose real mutation needs a query client this spec lacks.
    useToggleStar: () => ({ mutate: vi.fn() })
}))
const updateMutate = vi.fn()
// Invokes onSuccess so the component can re-baseline (clear dirty) after a save.
const replaceMutate = vi.fn((_payload: unknown, opts?: { onSuccess?: () => void }) =>
    opts?.onSuccess?.()
)
const coverMutate = vi.fn((_payload: unknown, opts?: { onSuccess?: () => void }) =>
    opts?.onSuccess?.()
)
const deleteMutate = vi.fn((_id: unknown, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.())
const updateAsync = vi.fn(() => Promise.resolve())
const replaceAsync = vi.fn(() => Promise.resolve())
const coverAsync = vi.fn(() => Promise.resolve())

const playAlbum = vi.fn()
const addMultipleToQueue = vi.fn()
const enqueueAndPlayIfIdle = vi.fn()
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ playAlbum, addMultipleToQueue, enqueueAndPlayIfIdle })
}))

// null = auth "none" (no per-user identity); the view treats that as owner.
const currentUser = ref<{ login: string; role: string } | null>(null)
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({ currentUser })
}))

const mockGetGeneratedCoverPreviewUrl = vi.fn()
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (id: string, size?: number) => `/cover/${id}?size=${size}`,
        scrobble: vi.fn(),
        getGeneratedCoverPreviewUrl: (...args: unknown[]) => mockGetGeneratedCoverPreviewUrl(...args)
    }
}))
vi.mock('sortablejs', () => ({ default: { create: () => ({ destroy: vi.fn() }) } }))

// Auto-accept the delete confirmation.
vi.mock('primevue/useconfirm', () => ({
    useConfirm: () => ({ require: (opts: { accept: () => void }) => opts.accept() })
}))

const push = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ back: vi.fn(), push }),
    onBeforeRouteLeave: vi.fn()
}))

import PlaylistDetailView from '@/views/PlaylistDetailView.vue'

const song = (id: string) => ({ id, title: `Song ${id}`, artist: 'A', album: 'Al', duration: 60 })

const mountView = () =>
    mount(PlaylistDetailView, {
        props: { id: 'pl1' },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            // vue-router is mocked, so RouterLink (used by GenreTrackRow's album
            // link) isn't registered — stub it to a plain anchor.
            stubs: {
                ConfirmDialog: true,
                RouterLink: { template: '<a><slot /></a>' },
                TrackActionSheet: {
                    name: 'TrackActionSheet',
                    props: ['song', 'visible'],
                    template: '<div />'
                },
                GenerateCoverDialog: {
                    name: 'GenerateCoverDialog',
                    props: ['visible', 'entityId', 'title'],
                    emits: ['update:visible', 'select'],
                    template: '<div class="generate-cover-dialog-stub" />'
                }
            }
        }
    })

const enterEdit = async (w: ReturnType<typeof mountView>) => {
    await w.find('.edit-action-edit').trigger('click')
}
// The Name field is the first <input> inside the hero's edit column
// (Description is a <textarea>, the cover picker lives in the flip back face).
const nameInput = (w: ReturnType<typeof mountView>) => w.find('.edit-only input')

beforeEach(() => {
    playlist.value = {
        id: 'pl1',
        name: 'My Mix',
        songCount: 3,
        entry: [song('1'), song('2'), song('3')]
    }
    currentUser.value = null
    updateMutate.mockReset()
    replaceMutate.mockClear()
    coverMutate.mockClear()
    deleteMutate.mockClear()
    updateAsync.mockReset().mockImplementation(() => Promise.resolve())
    replaceAsync.mockReset().mockImplementation(() => Promise.resolve())
    coverAsync.mockReset().mockImplementation(() => Promise.resolve())
    toastAdd.mockClear()
    playAlbum.mockReset()
    addMultipleToQueue.mockClear()
    enqueueAndPlayIfIdle.mockClear()
    push.mockClear()
    replaceIsPending.value = false
    coverIsPending.value = false
    // jsdom doesn't implement object URLs — stub them for the cover preview.
    global.URL.createObjectURL = vi.fn(() => 'blob:mock')
    global.URL.revokeObjectURL = vi.fn()
    mockGetGeneratedCoverPreviewUrl.mockClear()
    mockGetGeneratedCoverPreviewUrl.mockReturnValue('generated:preview')
})

describe('PlaylistDetailView', () => {
    it('shows only the back button in the header and the playlist name in the hero', () => {
        const w = mountView()
        expect(w.find('.scaffold-title h1').exists()).toBe(false)
        expect(w.find('.scaffold-back').exists()).toBe(true)
        expect(w.find('.hero-name').text()).toBe('My Mix')
    })

    it('view mode shows Play + pencil and no Save; edit mode shows Save/Cancel and hides Play', async () => {
        const w = mountView()
        expect(w.find('.hero-action-play').exists()).toBe(true)
        expect(w.find('.edit-action-edit').exists()).toBe(true)
        expect(w.find('.edit-action-save').exists()).toBe(false)

        await enterEdit(w)
        expect(w.find('.hero-header').classes()).toContain('editing')
        expect(w.find('.edit-action-save').exists()).toBe(true)
        expect(w.find('.edit-action-cancel').exists()).toBe(true)
        expect(w.find('.edit-action-delete').exists()).toBe(true)
        expect(w.find('.hero-action-play').exists()).toBe(false)
    })

    it('editing the name and saving persists it and exits edit mode', async () => {
        const w = mountView()
        await enterEdit(w)
        await nameInput(w).setValue('Road Trip')
        await w.find('.edit-action-save').trigger('click')
        await flushPromises()
        expect(updateAsync).toHaveBeenCalledWith(
            expect.objectContaining({ playlistId: 'pl1', name: 'Road Trip' })
        )
        // Saving leaves edit mode.
        expect(w.find('.hero-header').classes()).not.toContain('editing')
        expect(w.find('.hero-action-play').exists()).toBe(true)
    })

    it('Cancel discards the in-progress name edit and exits edit mode', async () => {
        const w = mountView()
        await enterEdit(w)
        await nameInput(w).setValue('Throwaway')
        await w.find('.edit-action-cancel').trigger('click')
        expect(updateMutate).not.toHaveBeenCalled()
        expect(w.find('.hero-header').classes()).not.toContain('editing')
        await enterEdit(w)
        expect((nameInput(w).element as HTMLInputElement).value).toBe('My Mix')
    })

    it('view mode renders album-style rows with covers; edit mode swaps in the editable list', async () => {
        const w = mountView()
        expect(w.find('.queue-edit-list').exists()).toBe(false)
        const rows = w.findAll('.track-list .genre-track-row')
        expect(rows).toHaveLength(3)
        expect(rows[0].find('.col-cover').exists()).toBe(true)

        await enterEdit(w)
        expect(w.find('.track-list').exists()).toBe(false)
        expect(w.find('.queue-edit-list').exists()).toBe(true)
        expect(w.findAll('.queue-edit-list .queue-row')).toHaveLength(3)
    })

    it('double-clicking a row in view mode appends that track to the queue', async () => {
        const w = mountView()
        await w.findAll('.track-list .genre-track-row')[1].trigger('dblclick')
        expect(enqueueAndPlayIfIdle).toHaveBeenCalledWith([song('2')])
        // Double-click appends — it must never replace the queue.
        expect(playAlbum).not.toHaveBeenCalled()
    })

    it('Save persists the working track order and exits edit mode', async () => {
        const w = mountView()
        await enterEdit(w)
        await w.find('[data-queue-index="0"] .delete-button').trigger('click')
        expect(w.findAll('.queue-edit-list .queue-row')).toHaveLength(2)

        await w.find('.edit-action-save').trigger('click')
        await flushPromises()
        expect(replaceAsync).toHaveBeenCalledWith(
            expect.objectContaining({ playlistId: 'pl1', songIds: ['2', '3'] })
        )
        expect(w.find('.hero-header').classes()).not.toContain('editing')
    })

    it('Save is disabled while a save is pending', async () => {
        const w = mountView()
        await enterEdit(w)
        await w.find('[data-queue-index="0"] .delete-button').trigger('click')
        replaceIsPending.value = true
        await w.vm.$nextTick()
        expect(w.find('.edit-action-save').attributes('disabled')).toBeDefined()
    })

    it('Save is disabled when the name is empty', async () => {
        const w = mountView()
        await enterEdit(w)
        await nameInput(w).setValue('   ')
        expect(w.find('.edit-action-save').attributes('disabled')).toBeDefined()
    })

    it('Play queues the current on-screen list', async () => {
        const w = mountView()
        await w.find('.hero-action-play').trigger('click')
        expect(playAlbum).toHaveBeenCalledWith([song('1'), song('2'), song('3')])
    })

    it('Add to queue enqueues the current on-screen list', async () => {
        const w = mountView()
        await w.find('.hero-action-queue').trigger('click')
        expect(addMultipleToQueue).toHaveBeenCalledWith([song('1'), song('2'), song('3')])
    })

    it('warns via beforeunload only when there are unsaved changes', async () => {
        const w = mountView()

        const clean = new Event('beforeunload', { cancelable: true })
        window.dispatchEvent(clean)
        expect(clean.defaultPrevented).toBe(false)

        await enterEdit(w)
        await w.find('[data-queue-index="0"] .delete-button').trigger('click')

        const dirty = new Event('beforeunload', { cancelable: true })
        window.dispatchEvent(dirty)
        expect(dirty.defaultPrevented).toBe(true)
    })

    it('renders a hero with the cover image and the owner in the meta row', () => {
        playlist.value = {
            id: 'pl1',
            name: 'My Mix',
            owner: 'admin',
            coverArt: 'pl1',
            songCount: 3,
            entry: [song('1'), song('2'), song('3')]
        }
        const w = mountView()
        expect(w.find('.hero-header').exists()).toBe(true)
        expect(w.find('.flip-front img').attributes('src')).toBe('/cover/pl1?size=250')
        expect(w.find('.meta-row').text()).toContain('admin')
    })

    it('staging a cover shows a local preview', async () => {
        const w = mountView()
        const file = new File(['x'], 'c.png', { type: 'image/png' })
        w.findComponent(FileUpload).vm.$emit('select', { files: [file] })
        await w.vm.$nextTick()
        expect(w.find('.flip-front img').attributes('src')).toContain('blob:')
    })

    it('Save uploads a staged cover via updatePlaylistCover', async () => {
        const w = mountView()
        const file = new File(['x'], 'c.png', { type: 'image/png' })
        w.findComponent(FileUpload).vm.$emit('select', { files: [file] })
        await w.vm.$nextTick()

        await enterEdit(w)
        await w.find('.edit-action-save').trigger('click')
        await flushPromises()
        expect(coverAsync).toHaveBeenCalledWith(
            expect.objectContaining({ playlistId: 'pl1', coverFile: file })
        )
    })

    it('Remove stages a cover clear that Save commits', async () => {
        const w = mountView()
        await w.find('.cover-remove').trigger('click')
        // Placeholder shown while a clear is staged; note explains the pending reset.
        expect(w.find('.flip-front img').exists()).toBe(false)
        expect(w.find('.cleared-note').exists()).toBe(true)

        await enterEdit(w)
        await w.find('.edit-action-save').trigger('click')
        await flushPromises()
        expect(coverAsync).toHaveBeenCalledWith(
            expect.objectContaining({ playlistId: 'pl1', coverClear: true })
        )
    })

    it('shows Generate button in edit mode and passes pick to mutation', async () => {
        const w = mountView()
        await enterEdit(w)

        const generateBtn = w.find('button[aria-label="Generate"]')
        expect(generateBtn.exists()).toBe(true)

        await generateBtn.trigger('click')
        await flushPromises()

        const dialog = w.findComponent({ name: 'GenerateCoverDialog' })
        expect(dialog.props('visible')).toBe(true)
        expect(dialog.props('entityId')).toBe('pl1')

        dialog.vm.$emit('select', { style: 'waves', variation: 7 })
        await flushPromises()

        expect(mockGetGeneratedCoverPreviewUrl).toHaveBeenCalledWith({
            id: 'pl1',
            style: 'waves',
            variation: 7,
            size: 512
        })

        await w.find('.edit-action-save').trigger('click')
        await flushPromises()
        expect(coverAsync).toHaveBeenCalledWith(
            expect.objectContaining({
                playlistId: 'pl1',
                coverFile: undefined,
                coverClear: undefined,
                generate: { style: 'waves', variation: 7 }
            })
        )
    })

    it('keeps edit mode open when a save fails', async () => {
        updateAsync.mockRejectedValueOnce(new Error('boom'))
        const w = mountView()
        await enterEdit(w)
        await nameInput(w).setValue('New Name')
        await w.find('.edit-action-save').trigger('click')
        await flushPromises()
        expect(w.find('.hero-header').classes()).toContain('editing')
        expect(w.find('.edit-action-save').exists()).toBe(true)
        expect(toastAdd).toHaveBeenCalledWith(expect.objectContaining({ severity: 'error' }))
    })

    it('Delete asks for confirmation, then deletes and navigates to /playlists', async () => {
        const w = mountView()
        await enterEdit(w)
        await w.find('.edit-action-delete').trigger('click')
        expect(deleteMutate).toHaveBeenCalledWith('pl1', expect.anything())
        expect(push).toHaveBeenCalledWith({ name: 'playlists' })
    })

    it('changing the playlist id reseeds the list and drops an in-progress name edit', async () => {
        const w = mountView()
        await enterEdit(w)
        await nameInput(w).setValue('Stale Draft')

        playlist.value = { id: 'pl2', name: 'Other Mix', songCount: 1, entry: [song('9')] }
        await w.setProps({ id: 'pl2' })

        expect(w.find('.hero-header').classes()).not.toContain('editing')
        expect(w.findAll('.track-list .genre-track-row')).toHaveLength(1)
        expect(updateMutate).not.toHaveBeenCalled()
        await enterEdit(w)
        expect((nameInput(w).element as HTMLInputElement).value).toBe('Other Mix')
    })
})

describe('PlaylistDetailView edit-mode selection bubble', () => {
    const selectRow = (w: ReturnType<typeof mountView>, index: number, ctrl = false) =>
        w
            .find(`.queue-edit-list [data-queue-index="${index}"]`)
            .trigger('click', ctrl ? { ctrlKey: true } : {})

    it('hides the "Playlist" eyebrow while editing', async () => {
        const w = mountView()
        expect(w.find('.eyebrow').text()).toBe('Playlist')
        await enterEdit(w)
        expect(w.find('.eyebrow').exists()).toBe(false)
    })

    it('reveals the bubble only once a track is selected in edit mode', async () => {
        const w = mountView()
        await enterEdit(w)
        expect(w.find('.edit-select').exists()).toBe(false)
        await selectRow(w, 0)
        expect(w.find('.edit-select').exists()).toBe(true)
        expect(w.find('.edit-select .es-count').text()).toContain('1 selected')
    })

    it('Move to top sends the selection to the front and Save persists the order', async () => {
        const w = mountView()
        await enterEdit(w)
        await selectRow(w, 2)
        await w.find('.edit-select .es-top').trigger('click')
        // Selection is cleared after the move, so the bubble goes away.
        expect(w.find('.edit-select').exists()).toBe(false)
        await w.find('.edit-action-save').trigger('click')
        await flushPromises()
        expect(replaceAsync).toHaveBeenCalledWith(
            expect.objectContaining({ playlistId: 'pl1', songIds: ['3', '1', '2'] })
        )
    })

    it('Move to bottom sends the selection to the end', async () => {
        const w = mountView()
        await enterEdit(w)
        await selectRow(w, 0)
        await w.find('.edit-select .es-bottom').trigger('click')
        await w.find('.edit-action-save').trigger('click')
        await flushPromises()
        expect(replaceAsync).toHaveBeenCalledWith(
            expect.objectContaining({ playlistId: 'pl1', songIds: ['2', '3', '1'] })
        )
    })

    it('Delete removes the selected tracks from the working list', async () => {
        const w = mountView()
        await enterEdit(w)
        await selectRow(w, 0)
        await selectRow(w, 2, true)
        await w.find('.edit-select .es-delete').trigger('click')
        expect(w.findAll('.queue-edit-list .queue-row')).toHaveLength(1)
        expect(w.find('.edit-select').exists()).toBe(false)
    })

    it('Clear dismisses the bubble without changing the list', async () => {
        const w = mountView()
        await enterEdit(w)
        await selectRow(w, 0)
        await w.find('.edit-select .es-clear').trigger('click')
        expect(w.find('.edit-select').exists()).toBe(false)
        expect(w.findAll('.queue-edit-list .queue-row')).toHaveLength(3)
    })

    it('leaving edit mode via Esc resets the selection (no lingering bubble)', async () => {
        const w = mountView()
        await enterEdit(w)
        await selectRow(w, 0)
        expect(w.find('.edit-select').exists()).toBe(true)

        // Esc while editing exits edit mode (EditActionBar listens on document).
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
        await flushPromises()

        expect(w.find('.hero-header').classes()).not.toContain('editing')
        // The selection is cleared, so neither the edit bubble nor the view-mode
        // selection bar lingers; the item actions come back instead.
        expect(w.find('.edit-select').exists()).toBe(false)
        expect(w.find('.hero-select').exists()).toBe(false)
        expect(w.find('.hero-actions').exists()).toBe(true)
    })

    it('Cancel also resets the selection when leaving edit mode', async () => {
        const w = mountView()
        await enterEdit(w)
        await selectRow(w, 0)
        await w.find('.edit-action-cancel').trigger('click')
        expect(w.find('.hero-header').classes()).not.toContain('editing')
        expect(w.find('.edit-select').exists()).toBe(false)
        expect(w.find('.hero-select').exists()).toBe(false)
    })
})

describe('PlaylistDetailView visibility toggle', () => {
    const selectRow = (w: ReturnType<typeof mountView>, index: number) =>
        w.find(`.queue-edit-list [data-queue-index="${index}"]`).trigger('click')

    it('offers the visibility toggle inside the edit form', async () => {
        const w = mountView()
        await enterEdit(w)
        // It belongs to the hero's edit slot (alongside name/description), shown
        // while editing — not a view-mode action. HeroHeader keeps both slots in
        // the DOM and swaps them by CSS, so this asserts placement, not visibility.
        expect(w.find('.edit-only .edit-visibility').exists()).toBe(true)
    })

    it('swaps the visibility toggle for the selection bubble once a track is selected', async () => {
        const w = mountView()
        await enterEdit(w)
        expect(w.find('.edit-visibility').exists()).toBe(true)
        expect(w.find('.edit-select').exists()).toBe(false)
        await selectRow(w, 0)
        expect(w.find('.edit-visibility').exists()).toBe(false)
        expect(w.find('.edit-select').exists()).toBe(true)
    })

    it('seeds the toggle from the playlist and labels a public playlist "Public"', async () => {
        playlist.value = {
            id: 'pl1',
            name: 'My Mix',
            public: true,
            songCount: 3,
            entry: [song('1'), song('2'), song('3')]
        }
        const w = mountView()
        await enterEdit(w)
        expect(w.find('.edit-visibility .ev-toggle').text()).toContain('Public')
    })

    it('toggling the visibility and saving persists the public flag', async () => {
        const w = mountView()
        await enterEdit(w)
        await w.find('.edit-visibility .ev-toggle').trigger('click')
        await w.find('.edit-action-save').trigger('click')
        await flushPromises()
        expect(updateAsync).toHaveBeenCalledWith(
            expect.objectContaining({ playlistId: 'pl1', public: true })
        )
    })

    it('does not call updatePlaylist when the visibility is left unchanged', async () => {
        const w = mountView()
        await enterEdit(w)
        // Reorder a track so there IS something to save, but never touch visibility.
        await w.find('[data-queue-index="0"] .delete-button').trigger('click')
        await w.find('.edit-action-save').trigger('click')
        await flushPromises()
        expect(updateAsync).not.toHaveBeenCalled()
    })
})

describe('PlaylistDetailView ownership gating', () => {
    it('marks a playlist owned by someone else read-only: not-mine icon, no edit button', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        playlist.value = {
            id: 'pl1',
            name: 'Bob Mix',
            owner: 'bob',
            public: true,
            songCount: 1,
            entry: [song('1')]
        }
        const w = mountView()
        expect(w.find('.not-mine-icon').exists()).toBe(true)
        expect(w.find('.edit-action-edit').exists()).toBe(false)
    })

    it('lets the owner edit: edit button present, no not-mine icon', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        playlist.value = {
            id: 'pl1',
            name: 'My Mix',
            owner: 'alice',
            songCount: 1,
            entry: [song('1')]
        }
        const w = mountView()
        expect(w.find('.not-mine-icon').exists()).toBe(false)
        expect(w.find('.edit-action-edit').exists()).toBe(true)
    })

    it('treats the lone visitor (auth "none") as owner — edit stays available', () => {
        currentUser.value = null
        playlist.value = {
            id: 'pl1',
            name: 'Mix',
            owner: 'admin',
            songCount: 1,
            entry: [song('1')]
        }
        const w = mountView()
        expect(w.find('.edit-action-edit').exists()).toBe(true)
        expect(w.find('.not-mine-icon').exists()).toBe(false)
    })
})
