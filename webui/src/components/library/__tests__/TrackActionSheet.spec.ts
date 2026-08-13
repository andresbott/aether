import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import PrimeVue from 'primevue/config'
import TrackActionSheet from '../TrackActionSheet.vue'
import type { Song } from '@/types/subsonic'

const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))

const addToQueue = vi.fn()
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ addToQueue })
}))

const toggleFavorite = vi.fn()
let starred = false
vi.mock('@/composables/useSongFavorite', () => ({
    useSongFavorite: () => ({
        isStarred: { value: starred },
        toggleFavorite
    })
}))

const updateMutate = vi.fn()
let playlists = ref<Array<{ id: string; name: string }>>([{ id: 'pl-1', name: 'Chill' }])
vi.mock('@/composables/useSubsonicQueries', () => ({
    usePlaylists: () => ({ data: playlists }),
    useUpdatePlaylist: () => ({ mutate: updateMutate })
}))

const SONG: Song = {
    id: 'tr-1',
    title: 'Karma Police',
    artist: 'Radiohead',
    albumId: 'al-9',
    artistId: 'ar-4'
} as Song

const mountSheet = async (song: Song | null = SONG) => {
    const wrapper = mount(TrackActionSheet, {
        props: { song, visible: false },
        global: { plugins: [PrimeVue] },
        attachTo: document.body
    })
    await wrapper.setProps({ visible: true })
    await nextTick()
    return wrapper
}

const actionByText = (text: string): HTMLElement | undefined =>
    Array.from(document.body.querySelectorAll<HTMLElement>('.sheet-action')).find((el) =>
        el.textContent?.includes(text)
    )

beforeEach(() => {
    vi.clearAllMocks()
    starred = false
    playlists = ref([{ id: 'pl-1', name: 'Chill' }])
    document.body.innerHTML = ''
})

describe('TrackActionSheet', () => {
    it('lists the actions for a song with album and artist ids', async () => {
        await mountSheet()
        const labels = Array.from(document.body.querySelectorAll('.sheet-action')).map((el) =>
            (el.textContent ?? '').trim()
        )
        expect(labels).toEqual([
            'Add to queue',
            'Add to favorites',
            'Add to playlist',
            'Go to album',
            'Go to artist'
        ])
    })

    it('hides the navigation actions without ids', async () => {
        await mountSheet({ ...SONG, albumId: undefined, artistId: undefined })
        expect(actionByText('Go to album')).toBeUndefined()
        expect(actionByText('Go to artist')).toBeUndefined()
    })

    it('add to queue enqueues and closes', async () => {
        const wrapper = await mountSheet()
        actionByText('Add to queue')?.click()
        expect(addToQueue).toHaveBeenCalledWith(SONG)
        expect(wrapper.emitted('update:visible')?.at(-1)).toEqual([false])
    })

    it('favorite reflects starred state and toggles', async () => {
        starred = true
        const wrapper = await mountSheet()
        const fav = actionByText('Remove from favorites')
        expect(fav).toBeTruthy()
        fav?.click()
        expect(toggleFavorite).toHaveBeenCalledOnce()
        expect(wrapper.emitted('update:visible')?.at(-1)).toEqual([false])
    })

    it('go to album navigates and closes', async () => {
        const wrapper = await mountSheet()
        actionByText('Go to album')?.click()
        expect(push).toHaveBeenCalledWith({ name: 'album', params: { id: 'al-9' } })
        expect(wrapper.emitted('update:visible')?.at(-1)).toEqual([false])
    })

    it('add to playlist shows the playlist face, picking one mutates and closes', async () => {
        const wrapper = await mountSheet()
        actionByText('Add to playlist')?.click()
        await nextTick()
        const pl = Array.from(document.body.querySelectorAll<HTMLElement>('.sheet-action')).find(
            (el) => el.textContent?.includes('Chill')
        )
        expect(pl).toBeTruthy()
        pl?.click()
        expect(updateMutate).toHaveBeenCalledWith(
            expect.objectContaining({ playlistId: 'pl-1', songIdsToAdd: ['tr-1'] })
        )
        expect(wrapper.emitted('update:visible')?.at(-1)).toEqual([false])
    })
})
