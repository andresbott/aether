import { describe, it, expect, vi } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { useEditForm } from '@/views/settings/metadata-editor/useEditForm'
import { useEditSession } from '@/composables/useEditSession'
import type { Track } from '@/types/metadata'

vi.mock('@tanstack/vue-query', async (orig) => ({
    ...(await orig<typeof import('@tanstack/vue-query')>()),
    useQueryClient: () => ({ invalidateQueries: vi.fn() })
}))
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))
vi.mock('@/composables/useMetadataEditor', async (orig) => ({
    ...(await orig<typeof import('@/composables/useMetadataEditor')>()),
    useApplyPicture: () => ({ mutateAsync: vi.fn(), isPending: { __v_isRef: true, value: false } }),
    useDeletePicture: () => ({ mutateAsync: vi.fn(), isPending: { __v_isRef: true, value: false } })
}))

const mkTrack = (over: Partial<Track> = {}): Track => ({
    path: 'a.mp3', name: 'a.mp3', title: 'Orig', artists: ['X'], album_artists: [],
    album: '', genres: [], year: 0, track_number: 0, disc_number: 0, disc_subtitle: '',
    compilation: false, mb_artist_ids: [''], mb_album_artist_ids: [], mb_recording_id: '',
    mb_release_id: '', mb_release_group_id: '', ...over
})

// harness mounts the composable so its computeds run inside an active scope,
// and exposes the return on the wrapper's vm for assertions.
function useForm(selection: Track[]) {
    let api!: ReturnType<typeof useEditForm>
    const Harness = defineComponent({
        setup() {
            const session = useEditSession(() => selection, () => 1)
            api = useEditForm(() => selection, session)
            return () => h('div')
        }
    })
    const wrapper = mount(Harness)
    return { api, wrapper }
}

describe('useEditForm scalars', () => {
    it('reads the shared title from the session and stages edits', async () => {
        const { api } = useForm([mkTrack({ title: 'Orig' })])
        const title = api.text('title')
        expect(title.value).toBe('Orig')

        title.value = 'New'
        await nextTick()
        expect(title.value).toBe('New')
        expect(api.isDirty('title')).toBe(true)
    })

    it('normalizes a scalar back to original (auto-unstages)', async () => {
        const { api } = useForm([mkTrack({ album: 'A' })])
        const album = api.text('album')
        album.value = 'B'
        expect(api.isDirty('album')).toBe(true)
        album.value = 'A'
        expect(api.isDirty('album')).toBe(false)
    })

    it('maps a 0 / mixed number to null and stages null as 0', () => {
        const { api } = useForm([mkTrack({ year: 0 })])
        const year = api.num('year')
        expect(year.value).toBe(null) // 0 renders as an empty box
        year.value = 1999
        expect(year.value).toBe(1999)
        year.value = null
        expect(api.isDirty('year')).toBe(false) // null -> 0 == original 0
    })

    it('marks placeholder "(multiple values)" for a mixed scalar', () => {
        const { api } = useForm([mkTrack({ album: 'A' }), mkTrack({ path: 'b.mp3', album: 'B' })])
        expect(api.placeholder('album').value).toBe('(multiple values)')
    })

    it('genres keep-each-track for a mixed selection emptied to []', () => {
        const a = mkTrack({ genres: ['Rock'] })
        const b = mkTrack({ path: 'b.mp3', genres: ['Jazz'] })
        const { api } = useForm([a, b])
        expect(api.genresMixed.value).toBe(true)
        api.genres.value = [] // empty over mixed -> stage nothing
        expect(api.isDirty('genres')).toBe(false)
        api.genres.value = ['Pop'] // non-empty -> overwrite all
        expect(api.isDirty('genres')).toBe(true)
    })
})
