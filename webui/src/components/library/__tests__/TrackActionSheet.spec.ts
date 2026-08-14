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
let playlistsLoading = ref(false)
// Captured so the gating test can assert the query is disabled until the sheet has
// been opened once.
const usePlaylistsOptions = vi.fn()
vi.mock('@/composables/useSubsonicQueries', () => ({
    usePlaylists: (options?: unknown) => {
        usePlaylistsOptions(options)
        return { data: playlists, isLoading: playlistsLoading }
    },
    useUpdatePlaylist: () => ({ mutate: updateMutate })
}))

const toastAdd = vi.fn()
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: toastAdd }) }))

const SONG: Song = {
    id: 'tr-1',
    title: 'Karma Police',
    artist: 'Radiohead',
    albumId: 'al-9',
    artistId: 'ar-4'
} as Song

const mountClosed = (song: Song | null = SONG) =>
    mount(TrackActionSheet, {
        props: { song, visible: false },
        global: { plugins: [PrimeVue] },
        attachTo: document.body
    })

const mountSheet = async (song: Song | null = SONG) => {
    const wrapper = mountClosed(song)
    await wrapper.setProps({ visible: true })
    await nextTick()
    return wrapper
}

const actionByText = (text: string): HTMLElement | undefined =>
    Array.from(document.body.querySelectorAll<HTMLElement>('.sheet-action')).find((el) =>
        el.textContent?.includes(text)
    )

/** The `enabled` option handed to usePlaylists on the most recent mount. */
const enabledOption = () => {
    const options = usePlaylistsOptions.mock.calls.at(-1)?.[0] as { enabled?: { value: boolean } }
    return options?.enabled
}

beforeEach(() => {
    vi.clearAllMocks()
    starred = false
    playlists = ref([{ id: 'pl-1', name: 'Chill' }])
    playlistsLoading = ref(false)
    document.body.innerHTML = ''
})

describe('TrackActionSheet', () => {
    it('lists the actions for a song with album and artist ids', async () => {
        await mountSheet()
        const labels = Array.from(document.body.querySelectorAll('.sheet-action')).map((el) =>
            (el.textContent ?? '').trim()
        )
        expect(labels).toEqual([
            'Play',
            'Add to queue',
            'Add to favorites',
            'Add to playlist',
            'Go to album',
            'Go to artist'
        ])
    })

    // Rows are tap targets, not tab stops: the sheet (reached via the tabbable
    // ⋮) is the keyboard path to starting playback at a specific track.
    it('play emits to the host and closes', async () => {
        const wrapper = await mountSheet()
        actionByText('Play')?.click()
        expect(wrapper.emitted('play')).toHaveLength(1)
        expect(wrapper.emitted('update:visible')?.at(-1)).toEqual([false])
    })

    // @primeuix's styled mode caps every bottom drawer at 10rem, which showed ~2 of
    // these actions. The opt-in class in _main.scss lifts that to auto/80dvh; it has
    // to land on the PANEL (.p-drawer), not the mask, or the override never matches.
    // The rule itself is pinned in assets/scss/__tests__/bottom-sheet.spec.ts.
    it('marks its panel as an auto-height bottom sheet', async () => {
        await mountSheet()
        const panel = document.body.querySelector('.p-drawer')
        expect(panel).toBeTruthy()
        expect(panel!.classList.contains('app-bottom-sheet')).toBe(true)
        expect(panel!.closest('.p-drawer-bottom')).toBeTruthy()
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

    const pickPlaylist = async () => {
        const wrapper = await mountSheet()
        actionByText('Add to playlist')?.click()
        await nextTick()
        const pl = Array.from(document.body.querySelectorAll<HTMLElement>('.sheet-action')).find(
            (el) => el.textContent?.includes('Chill')
        )
        expect(pl).toBeTruthy()
        pl?.click()
        return wrapper
    }

    /** The callbacks object handed to updatePlaylist.mutate as its second argument. */
    const mutateCallbacks = () =>
        updateMutate.mock.calls.at(-1)?.[1] as {
            onSuccess: () => void
            onError: (err: unknown) => void
        }

    it('add to playlist shows the playlist face, picking one mutates and closes', async () => {
        const wrapper = await pickPlaylist()
        expect(updateMutate).toHaveBeenCalledWith(
            expect.objectContaining({ playlistId: 'pl-1', songIdsToAdd: ['tr-1'] }),
            expect.anything()
        )
        expect(wrapper.emitted('update:visible')?.at(-1)).toEqual([false])
    })

    // The sheet closes on pick and the playlist is somewhere else, so without a toast
    // both outcomes look identical — a failed write read as a successful one.
    it('toasts the playlist name on a successful add', async () => {
        await pickPlaylist()
        mutateCallbacks().onSuccess()
        expect(toastAdd).toHaveBeenCalledWith(
            expect.objectContaining({ severity: 'success', summary: 'Added to Chill' })
        )
    })

    it('toasts the api message on a failed add', async () => {
        await pickPlaylist()
        mutateCallbacks().onError({ response: { data: { error: 'playlist is read-only' } } })
        expect(toastAdd).toHaveBeenCalledWith(
            expect.objectContaining({
                severity: 'error',
                summary: 'Failed to add to playlist',
                detail: 'playlist is read-only'
            })
        )
    })

    // Four views mount this sheet, and only touch can open it: an ungated query cost
    // one getPlaylists per desktop view visit for a face nobody there can reach.
    it('does not fetch playlists until the sheet has been opened once', async () => {
        const wrapper = mountClosed()
        expect(enabledOption()?.value).toBe(false)

        await wrapper.setProps({ visible: true })
        await nextTick()
        expect(enabledOption()?.value).toBe(true)

        // Latched: closing does not disable the query again, so reopening reuses the
        // list instead of refetching it.
        await wrapper.setProps({ visible: false })
        await nextTick()
        expect(enabledOption()?.value).toBe(true)
    })

    it('says the list is loading rather than claiming there are none', async () => {
        playlists = ref([])
        playlistsLoading = ref(true)
        await mountSheet()
        actionByText('Add to playlist')?.click()
        await nextTick()
        const empty = document.body.querySelector('.sheet-empty')
        expect(empty?.textContent?.trim()).toBe('Loading playlists…')
    })

    it('renders without error when song is null and clicking actions does not call mocks', async () => {
        const wrapper = await mountSheet(null)
        expect(actionByText('Add to queue')).toBeTruthy()
        expect(actionByText('Add to favorites')).toBeTruthy()
        expect(actionByText('Add to playlist')).toBeTruthy()
        expect(actionByText('Go to album')).toBeUndefined()
        expect(actionByText('Go to artist')).toBeUndefined()

        actionByText('Add to queue')?.click()
        expect(addToQueue).not.toHaveBeenCalled()

        actionByText('Add to favorites')?.click()
        expect(toggleFavorite).not.toHaveBeenCalled()

        actionByText('Play')?.click()
        expect(wrapper.emitted('play')).toBeUndefined()
    })
})
