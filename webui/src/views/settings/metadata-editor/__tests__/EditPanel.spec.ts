import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import EditPanel from '@/views/settings/metadata-editor/EditPanel.vue'
import { useEditSession, buildTrackPatch, type EditSession } from '@/composables/useEditSession'
import type { PatchFields, Track } from '@/types/metadata'

vi.mock('@/components/library/MusicBrainzArtistPicker.vue', () => ({
    default: {
        name: 'MusicBrainzArtistPicker',
        props: ['visible', 'artistName', 'currentMbid'],
        emits: ['select', 'update:visible'],
        template: '<div />'
    }
}))

vi.mock('@/components/library/MusicBrainzAlbumPicker.vue', () => ({
    default: {
        name: 'MusicBrainzAlbumPicker',
        props: ['visible', 'albumName', 'currentReleaseMbid', 'currentReleaseGroupMbid'],
        emits: ['select', 'update:visible'],
        template: '<div />'
    }
}))

// The pictures section has its own spec; stub it here so EditPanel tests
// don't need the pictures endpoint plumbing.
vi.mock('@/views/settings/metadata-editor/PicturesSection.vue', () => ({
    default: {
        name: 'PicturesSection',
        props: ['selection', 'libraryId', 'session', 'releaseMbid', 'releaseGroupMbid'],
        template: '<div class="pictures-section-stub" />'
    }
}))

// The raw editor has its own spec; stub it here so EditPanel tests don't need
// the raw-tags query plumbing.
vi.mock('@/views/settings/metadata-editor/RawEditPanel.vue', () => ({
    default: {
        name: 'RawEditPanel',
        props: ['selection', 'libraryId', 'session'],
        template: '<div class="raw-edit-panel-stub" />'
    }
}))

// The session composable calls useApplyPicture()/useDeletePicture() plus the
// query client and toast at setup; keep the real pure helpers but stub the
// side-effecting pieces so no vue-query/toast providers are needed.
const applyPictureSpy = vi.hoisted(() => vi.fn())
const deletePictureSpy = vi.hoisted(() => vi.fn())
vi.mock('@/composables/useMetadataEditor', async (importActual) => {
    const actual = await importActual<typeof import('@/composables/useMetadataEditor')>()
    return {
        ...actual,
        useApplyPicture: () => ({
            mutateAsync: applyPictureSpy,
            isPending: { __v_isRef: true, value: false }
        }),
        useDeletePicture: () => ({
            mutateAsync: deletePictureSpy,
            isPending: { __v_isRef: true, value: false }
        })
    }
})
vi.mock('@tanstack/vue-query', async (importActual) => {
    const actual = await importActual<typeof import('@tanstack/vue-query')>()
    return {
        ...actual,
        useQueryClient: () => ({ invalidateQueries: vi.fn() })
    }
})
vi.mock('primevue/usetoast', () => ({
    useToast: () => ({ add: vi.fn() })
}))

const stubs = {
    InputText: {
        props: ['modelValue', 'disabled'],
        template:
            '<input :disabled="disabled" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
    },
    // Mirrors InputNumber's nullable model: an emptied box emits null, not 0.
    InputNumber: {
        props: ['modelValue', 'disabled', 'placeholder'],
        template:
            '<input :disabled="disabled" :placeholder="placeholder" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value === \'\' ? null : Number($event.target.value))" />'
    },
    Checkbox: {
        props: ['modelValue', 'indeterminate'],
        template:
            '<input type="checkbox" :checked="modelValue" :data-indeterminate="indeterminate" :data-test="$attrs[\'data-test\']" @change="$emit(\'update:modelValue\', indeterminate ? true : !modelValue)" />'
    },
    // The chip input; tests drive it by emitting update:modelValue with the
    // full genre list, as the real AutoComplete (multiple) does.
    AutoComplete: {
        name: 'AutoComplete',
        props: ['modelValue', 'multiple', 'typeahead', 'disabled'],
        template: '<div class="genres-autocomplete">{{ (modelValue ?? []).join(",") }}</div>'
    },
    // inheritAttrs:false stops the parent's @click from ALSO falling through as a
    // native listener (which would fire the handler twice); aria-label is bound
    // explicitly so [aria-label=...] queries still resolve.
    Button: {
        props: ['label', 'disabled'],
        inheritAttrs: false,
        template:
            '<button :disabled="disabled" :aria-label="$attrs[\'aria-label\']" :data-test="$attrs[\'data-test\']" @click="$emit(\'click\')">{{ label }}</button>'
    }
}

// Stands in for PrimeVue's v-tooltip and mirrors the bound text onto the
// element so tests can assert what the user would read on hover. Handles both
// binding shapes the real directive accepts: a bare string, and the options
// object used when a tooltip also needs a class (see the album grouping help).
type TooltipBinding = { value: unknown }

function tooltipText(binding: TooltipBinding): string {
    const v = binding.value
    if (v && typeof v === 'object' && 'value' in v) {
        return String((v as { value: unknown }).value ?? '')
    }
    return String(v ?? '')
}

function tooltipClass(binding: TooltipBinding): string {
    const v = binding.value
    if (v && typeof v === 'object' && 'class' in v) {
        return String((v as { class: unknown }).class ?? '')
    }
    return ''
}

const tooltipRecorder = {
    mounted(el: HTMLElement, binding: TooltipBinding) {
        el.setAttribute('data-tooltip', tooltipText(binding))
        el.setAttribute('data-tooltip-class', tooltipClass(binding))
    },
    updated(el: HTMLElement, binding: TooltipBinding) {
        el.setAttribute('data-tooltip', tooltipText(binding))
        el.setAttribute('data-tooltip-class', tooltipClass(binding))
    }
}

const mkTrack = (over: Partial<Track> = {}): Track => ({
    path: 'a.mp3',
    name: 'a.mp3',
    title: '',
    artists: ['X'],
    album_artists: [],
    album: '',
    genres: [],
    year: 0,
    track_number: 0,
    disc_number: 0,
    disc_subtitle: '',
    compilation: false,
    mb_artist_ids: [''],
    mb_album_artist_ids: [],
    mb_recording_id: '',
    mb_release_id: '',
    mb_release_group_id: '',
    ...over
})

// mountPanel builds a real edit session over the selection and mounts the
// panel on top of it, mirroring how the view wires them together.
function mountPanel(selection: Track[], libraryId = 1, extraProps: Record<string, unknown> = {}) {
    const session = useEditSession(
        () => selection,
        () => libraryId
    )
    const wrapper = mount(EditPanel, {
        props: {
            selection,
            libraryId,
            session,
            canIdentify: false,
            isIdentifying: false,
            isIdentifyingAlbum: false,
            ...extraProps
        },
        global: { stubs, directives: { tooltip: tooltipRecorder } }
    })
    return { wrapper, session }
}

// stagedPatch resolves what would be written for one track given the session's
// current overlays — the session-model equivalent of the old save payload.
function stagedPatch(session: EditSession, track: Track): PatchFields {
    const overlay = session.overlays.value.get(track.path)
    return overlay ? buildTrackPatch(track, overlay) : {}
}

describe('EditPanel raw mode', () => {
    it('returns to the normal form view after a save', async () => {
        const { wrapper, session } = mountPanel([mkTrack()])

        await wrapper.get('[data-test="raw-toggle"]').trigger('click')
        expect(wrapper.find('.raw-edit-panel-stub').exists()).toBe(true)

        // A save cycle flips isSaving true then false; the panel should leave raw
        // mode once it settles.
        session.isSaving.value = true
        await nextTick()
        session.isSaving.value = false
        await nextTick()

        expect(wrapper.find('.raw-edit-panel-stub').exists()).toBe(false)
    })

    it('stays in raw mode until a save actually runs', async () => {
        const { wrapper } = mountPanel([mkTrack()])
        await wrapper.get('[data-test="raw-toggle"]').trigger('click')
        await nextTick()
        // No save has run, so raw mode persists.
        expect(wrapper.find('.raw-edit-panel-stub').exists()).toBe(true)
    })
})

describe('EditPanel pictures section', () => {
    it('mounts PicturesSection with the selection, session and release IDs', async () => {
        const track = mkTrack({
            path: 'album/t1.flac',
            mb_release_id: 'rel-1',
            mb_release_group_id: 'rg-1'
        })
        const { wrapper, session } = mountPanel([track], 3)
        const section = wrapper.findComponent({ name: 'PicturesSection' })
        expect(section.exists()).toBe(true)
        expect(section.props('selection')).toEqual([track])
        expect(section.props('libraryId')).toBe(3)
        expect(section.props('session')).toBe(session)
        expect(section.props('releaseMbid')).toBe('rel-1')
        expect(section.props('releaseGroupMbid')).toBe('rg-1')
    })

    it('hides PicturesSection in raw mode', async () => {
        const { wrapper } = mountPanel([mkTrack()], 3)
        await wrapper.find('[data-test="raw-toggle"]').trigger('click')
        expect(wrapper.findComponent({ name: 'PicturesSection' }).exists()).toBe(false)
    })
})

describe('EditPanel song section', () => {
    it('keeps the title input visible but disabled in a multi-track selection', () => {
        const { wrapper } = mountPanel([mkTrack(), mkTrack({ path: 'b.mp3' })])
        const title = wrapper.find('[data-test="song-block"] input.field-title')
        expect(title.exists()).toBe(true)
        expect(title.attributes('disabled')).toBeDefined()
    })

    it('enables the title input for a single-track selection', () => {
        const { wrapper } = mountPanel([mkTrack()])
        expect(
            wrapper.find('[data-test="song-block"] input.field-title').attributes('disabled')
        ).toBeUndefined()
    })

    it('renders the recording ID input and stages edits to it', async () => {
        const track = mkTrack({ mb_recording_id: 'old-rec' })
        const { wrapper, session } = mountPanel([track])
        const input = wrapper.find('input.field-mbid')
        expect((input.element as HTMLInputElement).value).toBe('old-rec')
        await input.setValue('new-rec')
        expect(stagedPatch(session, track).mb_recording_id).toBe('new-rec')
    })
})

describe('EditPanel album section', () => {
    // Album identity in the store is (album name, album artist, release ID) —
    // see store.FindOrCreateAlbum. An empty release ID is a value like any
    // other, not a wildcard, so a partially-tagged album splits into two rows in
    // the library. That trap is invisible from the form, hence the marker.
    it('explains the album grouping rules on the album name field', () => {
        const { wrapper } = mountPanel([mkTrack()])
        const marker = wrapper.find('[data-test="album-grouping-help"]')
        expect(marker.exists()).toBe(true)
        expect(marker.classes()).toContain('pi-exclamation-circle')
        // Prose needs more room than the theme's default tooltip width.
        expect(marker.attributes('data-tooltip-class')).toBe('wide-tooltip')

        const text = marker.attributes('data-tooltip') ?? ''
        // The three grouping keys, and what an empty release ID does.
        expect(text).toContain('album name')
        expect(text).toContain('album artist')
        expect(text).toContain('Release ID')
        expect(text).toContain('empty')
        expect(text).toContain('splits the album')
    })

    it('widens the album artists help tooltip too', () => {
        const { wrapper } = mountPanel([mkTrack()])
        const marker = wrapper.find('[data-test="album-artists-help"]')
        expect(marker.attributes('data-tooltip-class')).toBe('wide-tooltip')
        expect(marker.attributes('data-tooltip')).toContain('filed under')
    })
})

describe('EditPanel staging and undo', () => {
    it('marks a field dirty and stages its value on edit', async () => {
        const track = mkTrack()
        const { wrapper, session } = mountPanel([track])
        await wrapper.find('input.album-name').setValue('New Album')
        expect(stagedPatch(session, track).album).toBe('New Album')
        expect(wrapper.find('[data-test="undo-album"]').exists()).toBe(true)
    })

    it('typing the original value back clears the staged field', async () => {
        const track = mkTrack({ album: 'Original' })
        const { wrapper, session } = mountPanel([track])
        const input = wrapper.find('input.album-name')
        await input.setValue('Changed')
        expect(session.hasStagedChanges.value).toBe(true)
        await input.setValue('Original')
        expect(session.hasStagedChanges.value).toBe(false)
    })

    it('the per-field undo button reverts to the original value', async () => {
        const track = mkTrack({ album: 'Original' })
        const { wrapper, session } = mountPanel([track])
        const input = wrapper.find('input.album-name')
        await input.setValue('Changed')
        await wrapper.find('[data-test="undo-album"]').trigger('click')
        expect(session.hasStagedChanges.value).toBe(false)
        expect((wrapper.find('input.album-name').element as HTMLInputElement).value).toBe(
            'Original'
        )
    })

    it('stages a mass edit onto every selected track', async () => {
        const a = mkTrack({ path: 'a.mp3' })
        const b = mkTrack({ path: 'b.mp3' })
        const { wrapper, session } = mountPanel([a, b])
        await wrapper.find('input.album-name').setValue('Both')
        expect(stagedPatch(session, a).album).toBe('Both')
        expect(stagedPatch(session, b).album).toBe('Both')
        expect(session.stagedPaths.value.has('a.mp3')).toBe(true)
        expect(session.stagedPaths.value.has('b.mp3')).toBe(true)
    })
})

describe('EditPanel artist pairs', () => {
    it('renders one pair per artist with a blank ID input (no em dash)', () => {
        const { wrapper } = mountPanel([mkTrack()])
        expect(wrapper.text()).not.toContain('—')
        expect(wrapper.findAll('input.pair-name')).toHaveLength(1)
        const mbid = wrapper.find('input.pair-mbid')
        expect((mbid.element as HTMLInputElement).value).toBe('')
    })

    it('stages only artist_mbids (not names) when just an ID is typed', async () => {
        const track = mkTrack()
        const { wrapper, session } = mountPanel([track])
        await wrapper.find('input.pair-mbid').setValue('id-typed')
        const payload = stagedPatch(session, track)
        expect(payload.artist_mbids).toEqual({ X: 'id-typed' })
        expect(payload.artists).toBeUndefined()
    })

    it('adds an artist pair and stages the new name list plus its complete ID map', async () => {
        const track = mkTrack({ artists: ['X'], mb_artist_ids: ['id-x'] })
        const { wrapper, session } = mountPanel([track])
        const addBtn = wrapper.findAll('button').find((b) => b.text() === 'Add artist')
        await addBtn!.trigger('click')
        const names = wrapper.findAll('input.pair-name')
        expect(names).toHaveLength(2)
        await names[1].setValue('New Artist')
        const payload = stagedPatch(session, track)
        expect(payload.artists).toEqual(['X', 'New Artist'])
        // complete map rebuilds the aligned tag; empty IDs travel as ''
        expect(payload.artist_mbids).toEqual({ X: 'id-x', 'New Artist': '' })
    })

    it('removes an artist pair and rewrites the name list', async () => {
        const track = mkTrack({ artists: ['X', 'Y'], mb_artist_ids: ['id-x', 'id-y'] })
        const { wrapper, session } = mountPanel([track])
        const removeBtn = wrapper.findAll('button[aria-label="Remove artist"]')
        await removeBtn[1].trigger('click')
        const payload = stagedPatch(session, track)
        expect(payload.artists).toEqual(['X'])
        expect(payload.artist_mbids).toEqual({ X: 'id-x' })
    })

    it('fills both name and ID from the picker (flow b)', async () => {
        const track = mkTrack()
        const { wrapper, session } = mountPanel([track])
        await wrapper.find('button[aria-label="Search MusicBrainz"]').trigger('click')
        const picker = wrapper.findComponent({ name: 'MusicBrainzArtistPicker' })
        picker.vm.$emit('select', { name: 'The Beatles', mbid: 'id-b' })
        await wrapper.vm.$nextTick()
        const payload = stagedPatch(session, track)
        expect(payload.artists).toEqual(['The Beatles'])
        expect(payload.artist_mbids).toEqual({ 'The Beatles': 'id-b' })
    })

    it('supports album-artist pairs symmetrically', async () => {
        const track = mkTrack({ album_artists: ['Y'], mb_album_artist_ids: [''] })
        const { wrapper, session } = mountPanel([track])
        // second pair-mbid input belongs to the album-artist block
        const mbidInputs = wrapper.findAll('input.pair-mbid')
        expect(mbidInputs).toHaveLength(2)
        await mbidInputs[1].setValue('id-y')
        const payload = stagedPatch(session, track)
        expect(payload.album_artist_mbids).toEqual({ Y: 'id-y' })
    })

    it('the artists reset button unstages the whole credit list', async () => {
        const track = mkTrack({ artists: ['X'], mb_artist_ids: ['id-x'] })
        const { wrapper, session } = mountPanel([track])
        await wrapper.find('input.pair-name').setValue('Renamed')
        expect(session.hasStagedChanges.value).toBe(true)
        await wrapper.find('[data-test="undo-artists"]').trigger('click')
        expect(session.hasStagedChanges.value).toBe(false)
        expect((wrapper.find('input.pair-name').element as HTMLInputElement).value).toBe('X')
    })

    describe('album MusicBrainz IDs', () => {
        it('renders the release and release-group ID inputs', () => {
            const { wrapper } = mountPanel([
                mkTrack({ mb_release_id: 'rel-1', mb_release_group_id: 'rg-1' })
            ])
            const ids = wrapper.findAll('input.album-mbid')
            expect(ids).toHaveLength(2)
            expect((ids[0].element as HTMLInputElement).value).toBe('rel-1')
            expect((ids[1].element as HTMLInputElement).value).toBe('rg-1')
        })

        it('stages only the release ID when it is typed manually', async () => {
            const track = mkTrack()
            const { wrapper, session } = mountPanel([track])
            await wrapper.findAll('input.album-mbid')[0].setValue('rel-typed')
            const payload = stagedPatch(session, track)
            expect(payload.mb_release_id).toBe('rel-typed')
            expect('mb_release_group_id' in payload).toBe(false)
            expect('album' in payload).toBe(false)
        })

        it('fills album name plus both IDs when a release is picked', async () => {
            const track = mkTrack()
            const { wrapper, session } = mountPanel([track])
            const picker = wrapper.findComponent({ name: 'MusicBrainzAlbumPicker' })
            picker.vm.$emit('select', {
                album: 'OK Computer',
                mbReleaseId: 'rel-b',
                mbReleaseGroupId: 'rg-b'
            })
            await wrapper.vm.$nextTick()
            const payload = stagedPatch(session, track)
            expect(payload.album).toBe('OK Computer')
            expect(payload.mb_release_id).toBe('rel-b')
            expect(payload.mb_release_group_id).toBe('rg-b')
        })

        it('fills year and album artists when the picker payload includes them', async () => {
            const track = mkTrack()
            const { wrapper, session } = mountPanel([track])
            const picker = wrapper.findComponent({ name: 'MusicBrainzAlbumPicker' })
            picker.vm.$emit('select', {
                year: 1997,
                albumArtists: [{ name: 'Radiohead', mbid: 'artist-1' }]
            })
            await wrapper.vm.$nextTick()
            const payload = stagedPatch(session, track)
            expect(payload.year).toBe(1997)
            expect(payload.album_artists).toEqual(['Radiohead'])
            expect(payload.album_artist_mbids).toEqual({ Radiohead: 'artist-1' })
            expect('album' in payload).toBe(false)
            expect('mb_release_id' in payload).toBe(false)
        })

        it('clears both IDs without touching the name when the match is cleared', async () => {
            const track = mkTrack({
                album: 'Keep',
                mb_release_id: 'rel-1',
                mb_release_group_id: 'rg-1'
            })
            const { wrapper, session } = mountPanel([track])
            const picker = wrapper.findComponent({ name: 'MusicBrainzAlbumPicker' })
            picker.vm.$emit('select', { mbReleaseId: '', mbReleaseGroupId: '' })
            await wrapper.vm.$nextTick()
            const payload = stagedPatch(session, track)
            expect(payload.mb_release_id).toBe('')
            expect(payload.mb_release_group_id).toBe('')
            expect('album' in payload).toBe(false)
        })
    })

    describe('disc fields', () => {
        it('renders disc number and disc subtitle inputs', () => {
            const { wrapper } = mountPanel([mkTrack({ disc_number: 2, disc_subtitle: 'CD 2' })])
            expect(wrapper.text()).toContain('Disc number')
            expect(wrapper.text()).toContain('Disc subtitle')
            expect(
                (wrapper.find('input.field-disc-number').element as HTMLInputElement).value
            ).toBe('2')
            expect(
                (wrapper.find('input.field-disc-subtitle').element as HTMLInputElement).value
            ).toBe('CD 2')
        })

        it('stages both disc fields when both are edited', async () => {
            const track = mkTrack()
            const { wrapper, session } = mountPanel([track])
            await wrapper.find('input.field-disc-number').setValue('2')
            await wrapper.find('input.field-disc-subtitle').setValue('Bonus Disc')
            const payload = stagedPatch(session, track)
            expect(payload.disc_number).toBe(2)
            expect(payload.disc_subtitle).toBe('Bonus Disc')
        })

        it('omits an untouched disc field from the patch', async () => {
            const track = mkTrack()
            const { wrapper, session } = mountPanel([track])
            // touch only disc number; disc subtitle must not appear in the patch
            await wrapper.find('input.field-disc-number').setValue('3')
            const payload = stagedPatch(session, track)
            expect(payload.disc_number).toBe(3)
            expect('disc_subtitle' in payload).toBe(false)
        })

        it('mass-edits disc number across a multi-track selection', async () => {
            const a = mkTrack({ path: 'a.mp3', disc_number: 1 })
            const b = mkTrack({ path: 'b.mp3', disc_number: 1 })
            const { wrapper, session } = mountPanel([a, b])
            await wrapper.find('input.field-disc-number').setValue('2')
            expect(stagedPatch(session, a).disc_number).toBe(2)
            expect(stagedPatch(session, b).disc_number).toBe(2)
        })
    })

    describe('genres', () => {
        it('prefills the shared genre list', () => {
            const { wrapper } = mountPanel([mkTrack({ genres: ['Rock', 'Jazz'] })])
            expect(wrapper.find('.genres-autocomplete').text()).toBe('Rock,Jazz')
        })

        it('stages an edited genre list', async () => {
            const track = mkTrack({ genres: ['Rock'] })
            const { wrapper, session } = mountPanel([track])
            wrapper
                .findComponent({ name: 'AutoComplete' })
                .vm.$emit('update:modelValue', ['Rock', 'Jazz'])
            await wrapper.vm.$nextTick()
            expect(stagedPatch(session, track).genres).toEqual(['Rock', 'Jazz'])
        })

        it('undoes a staged genre edit back to the original', async () => {
            const track = mkTrack({ genres: ['Rock'] })
            const { wrapper, session } = mountPanel([track])
            wrapper.findComponent({ name: 'AutoComplete' }).vm.$emit('update:modelValue', ['Pop'])
            await wrapper.vm.$nextTick()
            await wrapper.find('[data-test="undo-genres"]').trigger('click')
            expect(stagedPatch(session, track)).toEqual({})
            expect(wrapper.find('.genres-autocomplete').text()).toBe('Rock')
        })

        it('mass-overwrites genres on every selected track', async () => {
            const a = mkTrack({ path: 'a.mp3', genres: ['Rock'] })
            const b = mkTrack({ path: 'b.mp3', genres: ['Pop'] })
            const { wrapper, session } = mountPanel([a, b])
            expect(wrapper.find('[data-test="genres-mixed"]').exists()).toBe(true)
            wrapper
                .findComponent({ name: 'AutoComplete' })
                .vm.$emit('update:modelValue', ['Electronic'])
            await wrapper.vm.$nextTick()
            expect(stagedPatch(session, a).genres).toEqual(['Electronic'])
            expect(stagedPatch(session, b).genres).toEqual(['Electronic'])
        })

        it('an empty list over originally-mixed genres stages nothing', async () => {
            const a = mkTrack({ path: 'a.mp3', genres: ['Rock'] })
            const b = mkTrack({ path: 'b.mp3', genres: ['Pop'] })
            const { wrapper, session } = mountPanel([a, b])
            const auto = wrapper.findComponent({ name: 'AutoComplete' })
            auto.vm.$emit('update:modelValue', ['Electronic'])
            await wrapper.vm.$nextTick()
            auto.vm.$emit('update:modelValue', [])
            await wrapper.vm.$nextTick()
            expect(stagedPatch(session, a)).toEqual({})
            expect(stagedPatch(session, b)).toEqual({})
        })

        it('stages genres from the album picker payload', async () => {
            const track = mkTrack()
            const { wrapper, session } = mountPanel([track])
            const picker = wrapper.findComponent({ name: 'MusicBrainzAlbumPicker' })
            picker.vm.$emit('select', { genres: ['art rock', 'alternative rock'] })
            await wrapper.vm.$nextTick()
            expect(stagedPatch(session, track).genres).toEqual([
                'art rock',
                'alternative rock'
            ])
        })
    })

    describe('track number', () => {
        it('stages an edited track number on a single track', async () => {
            const track = mkTrack({ track_number: 1 })
            const { wrapper, session } = mountPanel([track])
            await wrapper.find('input.field-track-number').setValue('7')
            expect(stagedPatch(session, track).track_number).toBe(7)
        })

        it('is disabled and stages nothing in mass mode', async () => {
            const a = mkTrack({ path: 'a.mp3' })
            const b = mkTrack({ path: 'b.mp3' })
            const { wrapper, session } = mountPanel([a, b])
            const input = wrapper.find('input.field-track-number')
            expect(input.attributes('disabled')).toBeDefined()
            await input.setValue('7')
            expect(stagedPatch(session, a)).toEqual({})
            expect(stagedPatch(session, b)).toEqual({})
        })
    })

    describe('mixed-value placeholders', () => {
        // A mixed numeric field must show "(multiple values)": the buffer holds
        // null so InputNumber renders an empty box the placeholder shows through,
        // instead of a literal 0 that hides it.
        const cases: Array<[string, string, Partial<Track>, Partial<Track>]> = [
            ['disc number', 'input.field-disc-number', { disc_number: 1 }, { disc_number: 2 }],
            ['year', 'input.field-year', { year: 1997 }, { year: 2001 }],
            ['album name', 'input.album-name', { album: 'A' }, { album: 'B' }],
            [
                'disc subtitle',
                'input.field-disc-subtitle',
                { disc_subtitle: 'CD 1' },
                { disc_subtitle: 'CD 2' }
            ],
            [
                'release ID',
                'input.album-mbid',
                { mb_release_id: 'rel-a' },
                { mb_release_id: 'rel-b' }
            ]
        ]

        it.each(cases)('shows the mixed placeholder for %s', (_name, selector, a, b) => {
            const { wrapper } = mountPanel([
                mkTrack({ path: 'a.mp3', ...a }),
                mkTrack({ path: 'b.mp3', ...b })
            ])
            const input = wrapper.find(selector)
            expect((input.element as HTMLInputElement).value).toBe('')
            expect(input.attributes('placeholder')).toBe('(multiple values)')
        })

        it('shows the mixed placeholder for the release-group ID', () => {
            const { wrapper } = mountPanel([
                mkTrack({ path: 'a.mp3', mb_release_group_id: 'rg-a' }),
                mkTrack({ path: 'b.mp3', mb_release_group_id: 'rg-b' })
            ])
            const input = wrapper.findAll('input.album-mbid')[1]
            expect((input.element as HTMLInputElement).value).toBe('')
            expect(input.attributes('placeholder')).toBe('(multiple values)')
        })

        it('renders an empty numeric box rather than 0 when the tag is unset', () => {
            const { wrapper } = mountPanel([mkTrack({ disc_number: 0, year: 0 })])
            expect(
                (wrapper.find('input.field-disc-number').element as HTMLInputElement).value
            ).toBe('')
            expect((wrapper.find('input.field-year').element as HTMLInputElement).value).toBe('')
        })

        it('clearing a numeric field stages 0 so the tag is removed', async () => {
            const track = mkTrack({ disc_number: 3 })
            const { wrapper, session } = mountPanel([track])
            await wrapper.find('input.field-disc-number').setValue('')
            expect(stagedPatch(session, track).disc_number).toBe(0)
        })

        // A checkbox has no placeholder, so mixed renders as indeterminate.
        it('marks a mixed compilation checkbox indeterminate with a note', () => {
            const { wrapper } = mountPanel([
                mkTrack({ path: 'a.mp3', compilation: true }),
                mkTrack({ path: 'b.mp3', compilation: false })
            ])
            expect(
                wrapper.find('[data-test="compilation-input"]').attributes('data-indeterminate')
            ).toBe('true')
            expect(wrapper.find('[data-test="compilation-mixed"]').exists()).toBe(true)
        })

        it('leaves a shared compilation checkbox determinate', () => {
            const { wrapper } = mountPanel([
                mkTrack({ path: 'a.mp3', compilation: true }),
                mkTrack({ path: 'b.mp3', compilation: true })
            ])
            expect(
                wrapper.find('[data-test="compilation-input"]').attributes('data-indeterminate')
            ).toBe('false')
            expect(wrapper.find('[data-test="compilation-mixed"]').exists()).toBe(false)
        })

        it('clicking a mixed compilation checkbox stages true on every track', async () => {
            const a = mkTrack({ path: 'a.mp3', compilation: true })
            const b = mkTrack({ path: 'b.mp3', compilation: false })
            const { wrapper, session } = mountPanel([a, b])
            await wrapper.find('[data-test="compilation-input"]').trigger('change')
            // a already had true, so only b gets an overlay entry
            expect(stagedPatch(session, a)).toEqual({})
            expect(stagedPatch(session, b).compilation).toBe(true)
        })
    })

    describe('mixed multi-selection', () => {
        const mkSelection = () => [
            mkTrack({ path: 'a.mp3', artists: ['A'], mb_artist_ids: [''] }),
            mkTrack({ path: 'b.mp3', artists: ['B'], mb_artist_ids: [''] })
        ]

        it('starts with a blank artist list and shows the mixed note', () => {
            const { wrapper } = mountPanel(mkSelection())
            // no artist rows to start; the whole list is overwritten by adding artists
            expect(wrapper.findAll('input.pair-name')).toHaveLength(0)
            expect(wrapper.text()).toContain('Selected tracks have different artists')
        })

        it('overwrites the whole list for every track when artists are added', async () => {
            const selection = mkSelection()
            const { wrapper, session } = mountPanel(selection)
            const addBtn = wrapper.findAll('button').find((b) => b.text() === 'Add artist')
            await addBtn!.trigger('click')
            await wrapper.find('input.pair-name').setValue('New Artist')
            for (const track of selection) {
                const payload = stagedPatch(session, track)
                expect(payload.artists).toEqual(['New Artist'])
                expect(payload.artist_mbids).toEqual({ 'New Artist': '' })
            }
        })

        it('leaves artists untouched while staging other edited fields', async () => {
            const selection = mkSelection()
            const { wrapper, session } = mountPanel(selection)
            // edit only the disc subtitle; artists must not appear in the patch
            await wrapper.find('input.field-disc-subtitle').setValue('CD 2')
            const payload = stagedPatch(session, selection[0])
            expect(payload.disc_subtitle).toBe('CD 2')
            expect('artists' in payload).toBe(false)
            expect('artist_mbids' in payload).toBe(false)
        })
    })
})

describe('EditPanel raw mode', () => {
    it('the Raw button swaps the form body for the raw editor and back', async () => {
        const { wrapper } = mountPanel([mkTrack()])
        expect(wrapper.find('[data-test="song-block"]').exists()).toBe(true)
        expect(wrapper.findComponent({ name: 'RawEditPanel' }).exists()).toBe(false)

        await wrapper.find('[data-test="raw-toggle"]').trigger('click')
        expect(wrapper.find('[data-test="song-block"]').exists()).toBe(false)
        expect(wrapper.findComponent({ name: 'RawEditPanel' }).exists()).toBe(true)

        await wrapper.find('[data-test="raw-toggle"]').trigger('click')
        expect(wrapper.find('[data-test="song-block"]').exists()).toBe(true)
    })
})

describe('EditPanel identify button', () => {
    it('is visible but disabled when the server lacks the capability', () => {
        const { wrapper } = mountPanel([mkTrack()], 1, { canIdentify: false })
        const btn = wrapper.find('[data-test="identify-button"]')
        expect(btn.exists()).toBe(true)
        expect(btn.attributes('disabled')).toBeDefined()
    })

    it('shows the server reason for the disabled state', () => {
        const { wrapper } = mountPanel([mkTrack()], 1, {
            canIdentify: false,
            identifyUnavailableReason: 'fpcalc is missing; install libchromaprint-tools'
        })
        expect(wrapper.find('.identify-wrap').attributes('data-tooltip')).toBe(
            'fpcalc is missing; install libchromaprint-tools'
        )
    })

    it('is disabled when nothing in the selection can be fingerprinted', () => {
        const bad = mkTrack({ path: 'bad.mp3', error: 'read failed' })
        const { wrapper } = mountPanel([bad], 1, { canIdentify: true })
        const btn = wrapper.find('[data-test="identify-button"]')
        expect(btn.attributes('disabled')).toBeDefined()
        expect(wrapper.find('.identify-wrap').attributes('data-tooltip')).toContain(
            'nothing to fingerprint'
        )
    })

    it('is hidden while raw mode is active', async () => {
        const { wrapper } = mountPanel([mkTrack()], 1, { canIdentify: true })
        expect(wrapper.find('[data-test="identify-button"]').exists()).toBe(true)
        await wrapper.find('[data-test="raw-toggle"]').trigger('click')
        expect(wrapper.find('[data-test="identify-button"]').exists()).toBe(false)
        await wrapper.find('[data-test="raw-toggle"]').trigger('click')
        expect(wrapper.find('[data-test="identify-button"]').exists()).toBe(true)
    })

    it('emits identify with the non-error selection', async () => {
        const good = mkTrack({ path: 'good.mp3' })
        const bad = mkTrack({ path: 'bad.mp3', error: 'read failed' })
        const { wrapper } = mountPanel([good, bad], 1, { canIdentify: true })
        await wrapper.find('[data-test="identify-button"]').trigger('click')
        const emitted = wrapper.emitted('identify')
        expect(emitted).toHaveLength(1)
        expect(emitted![0][0]).toEqual([good])
    })
})

describe('EditPanel album identify', () => {
    it('hides the album button for a single selected track', () => {
        const { wrapper } = mountPanel([mkTrack({ path: 'a.mp3' })])
        expect(wrapper.find('[data-test="identify-album-button"]').exists()).toBe(false)
    })

    it('shows the album button above one selected track', () => {
        const { wrapper } = mountPanel([
            mkTrack({ path: 'a.mp3' }),
            mkTrack({ path: 'b.mp3' })
        ])
        expect(wrapper.find('[data-test="identify-album-button"]').exists()).toBe(true)
    })

    it('emits identify-album with the readable tracks', async () => {
        const good = mkTrack({ path: 'a.mp3' })
        const other = mkTrack({ path: 'b.mp3' })
        const broken = mkTrack({ path: 'c.mp3', error: 'unreadable' })
        const { wrapper } = mountPanel([good, other, broken], 1, { canIdentify: true })

        await wrapper.find('[data-test="identify-album-button"]').trigger('click')
        const emitted = wrapper.emitted('identify-album')![0][0] as Track[]
        expect(emitted.map((t) => t.path)).toEqual(['a.mp3', 'b.mp3'])
    })

    it('disables the album button when the server cannot identify', () => {
        const { wrapper } = mountPanel(
            [mkTrack({ path: 'a.mp3' }), mkTrack({ path: 'b.mp3' })],
            1,
            { canIdentify: false }
        )
        expect(
            wrapper.find('[data-test="identify-album-button"]').attributes('disabled')
        ).toBeDefined()
    })

    it('disables the album button when fewer than two tracks are readable', () => {
        const { wrapper } = mountPanel([
            mkTrack({ path: 'a.mp3' }),
            mkTrack({ path: 'b.mp3', error: 'unreadable' })
        ])
        // Visible (two selected) but unusable: one readable file is not an album.
        expect(
            wrapper.find('[data-test="identify-album-button"]').attributes('disabled')
        ).toBeDefined()
    })
})
