import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'

const toastAdd = vi.fn()
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: toastAdd }) }))

// mutate(vars, opts) invokes onSuccess so the toast + clear paths are exercised.
const updateMutate = vi.fn((_vars: unknown, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.())
const createMutate = vi.fn((_vars: unknown, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.())
const starMutate = vi.fn((_vars: unknown, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.())
const playlistsData = ref<unknown[]>([
    { id: 'p1', name: 'Late Night', songCount: 42, coverArt: 'c1' },
    { id: 'p2', name: 'Focus Deep', songCount: 88 }
])
vi.mock('@/composables/useSubsonicQueries', () => ({
    usePlaylists: () => ({ data: playlistsData, isLoading: ref(false) }),
    useUpdatePlaylist: () => ({ mutate: updateMutate }),
    useCreatePlaylist: () => ({ mutate: createMutate, isPending: ref(false) }),
    useStarSongs: () => ({ mutate: starMutate, isPending: ref(false) })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (art: string, size: number) => `cover:${art}:${size}`
    }
}))
vi.mock('@/lib/apiError', () => ({ apiErrorMessage: (e: unknown) => String(e) }))

import HeroSelectionBar from '@/components/layout/HeroSelectionBar.vue'

const songs = [
    { id: 's1', title: 'One' },
    { id: 's2', title: 'Two' }
]

// Renders its slot inline so the picker content is queryable, and exposes the
// toggle/hide methods the component calls on its ref.
const PopoverStub = {
    name: 'Popover',
    template: '<div class="popover-stub"><slot /></div>',
    methods: {
        toggle(this: { $emit: (e: string) => void }) {
            this.$emit('show')
        },
        hide(this: { $emit: (e: string) => void }) {
            this.$emit('hide')
        }
    }
}

const mountBar = (props: Record<string, unknown> = {}) =>
    mount(HeroSelectionBar, {
        props: { count: songs.length, songs, ...props },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { Popover: PopoverStub }
        }
    })

beforeEach(() => {
    toastAdd.mockClear()
    updateMutate.mockClear()
    createMutate.mockClear()
    starMutate.mockClear()
})

describe('HeroSelectionBar', () => {
    it('shows the selected count', () => {
        const w = mountBar({ count: 3 })
        expect(w.find('.hs-count').text()).toContain('3')
        expect(w.find('.hs-count').text()).toContain('selected')
    })

    it('emits play / queue / clear from the strip', async () => {
        const w = mountBar()
        await w.find('.sel-play').trigger('click')
        await w.find('.sel-queue').trigger('click')
        await w.find('.sel-clear').trigger('click')
        expect(w.emitted('play')).toHaveLength(1)
        expect(w.emitted('queue')).toHaveLength(1)
        expect(w.emitted('clear')).toHaveLength(1)
    })

    it('adds every selected song to favorites', async () => {
        const w = mountBar()
        await w.find('.sel-favorite').trigger('click')
        expect(starMutate).toHaveBeenCalledTimes(1)
        expect(starMutate.mock.calls[0][0]).toEqual(['s1', 's2'])
        expect(toastAdd).toHaveBeenCalledWith(
            expect.objectContaining({ severity: 'success', summary: 'Added to favorites' })
        )
    })

    it('adds the selection to a chosen playlist, then clears', async () => {
        const w = mountBar()
        await w.find('.sel-add-playlist').trigger('click')
        await w.find('.sel-pl-item').trigger('click')
        expect(updateMutate.mock.calls[0][0]).toEqual({
            playlistId: 'p1',
            songIdsToAdd: ['s1', 's2']
        })
        expect(toastAdd).toHaveBeenCalledWith(
            expect.objectContaining({ severity: 'success', summary: 'Added to Late Night' })
        )
        expect(w.emitted('clear')).toHaveLength(1)
    })

    it('filters the playlist list by the search box', async () => {
        const w = mountBar()
        await w.find('.sel-add-playlist').trigger('click')
        expect(w.findAll('.sel-pl-item')).toHaveLength(2)
        await w.find('.pl-search input').setValue('focus')
        expect(w.findAll('.sel-pl-item')).toHaveLength(1)
        expect(w.find('.sel-pl-item').text()).toContain('Focus Deep')
    })

    it('creates a new playlist from the selection', async () => {
        const w = mountBar()
        await w.find('.sel-add-playlist').trigger('click')
        await w.find('.sel-pl-new').trigger('click')
        await w.find('.pl-new-form input').setValue('Road Trip')
        await w.find('.sel-pl-create').trigger('click')
        await flushPromises()
        expect(createMutate.mock.calls[0][0]).toEqual({
            name: 'Road Trip',
            songIds: ['s1', 's2']
        })
        expect(toastAdd).toHaveBeenCalledWith(
            expect.objectContaining({ severity: 'success', summary: 'Created Road Trip' })
        )
        expect(w.emitted('clear')).toHaveLength(1)
    })
})
