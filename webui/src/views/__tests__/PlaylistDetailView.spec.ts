import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'

const playlist = ref<any>(null)
vi.mock('@/composables/useSubsonicQueries', () => ({
    usePlaylist: () => ({ data: playlist, isLoading: ref(false), error: ref(null) }),
    useUpdatePlaylist: () => ({ mutate: updateMutate, isPending: ref(false) }),
    useDeletePlaylist: () => ({ mutate: vi.fn() }),
    useReplacePlaylistTracks: () => ({ mutate: replaceMutate, isPending: ref(false) })
}))
const updateMutate = vi.fn()
const replaceMutate = vi.fn()

const playAlbum = vi.fn()
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playAlbum }) }))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '' }
}))
vi.mock('sortablejs', () => ({ default: { create: () => ({ destroy: vi.fn() }) } }))
vi.mock('vue-router', () => ({ useRouter: () => ({ back: vi.fn(), push: vi.fn() }) }))

import PlaylistDetailView from '@/views/PlaylistDetailView.vue'

const song = (id: string) => ({ id, title: `Song ${id}`, artist: 'A', album: 'Al', duration: 60 })

const mountView = () =>
    mount(PlaylistDetailView, {
        props: { id: 'pl1' },
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

beforeEach(() => {
    playlist.value = { id: 'pl1', name: 'My Mix', songCount: 3, entry: [song('1'), song('2'), song('3')] }
    updateMutate.mockReset()
    replaceMutate.mockReset()
    playAlbum.mockReset()
})

describe('PlaylistDetailView', () => {
    it('shows the playlist name and an inline rename control beside the title', () => {
        const w = mountView()
        expect(w.find('.scaffold-title h1').text()).toBe('My Mix')
        expect(w.find('.rename-toggle').exists()).toBe(true)
    })

    it('inline rename submits the new name', async () => {
        const w = mountView()
        await w.find('.rename-toggle').trigger('click')
        const input = w.find('.rename-input input')
        await input.setValue('Road Trip')
        await input.trigger('keyup.enter')
        expect(updateMutate).toHaveBeenCalledWith(
            expect.objectContaining({ playlistId: 'pl1', name: 'Road Trip' }),
            expect.anything()
        )
    })

    it('entering edit mode shows the TrackEditList and Save/Cancel', async () => {
        const w = mountView()
        expect(w.find('.queue-edit-list').exists()).toBe(false)
        await w.find('.edit-toggle').trigger('click')
        expect(w.find('.queue-edit-list').exists()).toBe(true)
        expect(w.find('.edit-save').exists()).toBe(true)
        expect(w.find('.edit-cancel').exists()).toBe(true)
    })

    it('deleting in edit mode is local until Save', async () => {
        const w = mountView()
        await w.find('.edit-toggle').trigger('click')
        await w.find('[data-queue-index="1"] .delete-button').trigger('click')
        expect(replaceMutate).not.toHaveBeenCalled()
        expect(w.findAll('.queue-edit-list .queue-row')).toHaveLength(2)
    })

    it('Save persists the working order via replacePlaylistTracks', async () => {
        const w = mountView()
        await w.find('.edit-toggle').trigger('click')
        await w.find('[data-queue-index="0"] .delete-button').trigger('click')
        await w.find('.edit-save').trigger('click')
        expect(replaceMutate).toHaveBeenCalledWith(
            expect.objectContaining({ playlistId: 'pl1', songIds: ['2', '3'] }),
            expect.anything()
        )
    })

    it('Cancel discards local edits and leaves edit mode', async () => {
        const w = mountView()
        await w.find('.edit-toggle').trigger('click')
        await w.find('[data-queue-index="0"] .delete-button').trigger('click')
        await w.find('.edit-cancel').trigger('click')
        expect(w.find('.queue-edit-list').exists()).toBe(false)
        expect(replaceMutate).not.toHaveBeenCalled()
    })

    it('Play queues all entries', async () => {
        const w = mountView()
        await w.find('.play-all').trigger('click')
        expect(playAlbum).toHaveBeenCalledWith(playlist.value.entry)
    })
})
