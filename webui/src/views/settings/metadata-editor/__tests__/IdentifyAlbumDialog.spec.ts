import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

// Genres are looked up per release group when an option is selected; drive that
// through a spy so the specs can assert what was asked for and how often.
const genresMock = vi.fn()
vi.mock('@/lib/api/Artists', () => ({
    getReleaseGroupGenres: (...args: unknown[]) => genresMock(...args)
}))

import IdentifyAlbumDialog from '@/views/settings/metadata-editor/IdentifyAlbumDialog.vue'
import { ALL_IDENTIFY_FIELD_IDS, IDENTIFY_FIELDS } from '@/lib/identifyFields'
import { useReleaseGroupGenres } from '@/composables/useReleaseGroupGenres'
import type { AlbumOption, Track } from '@/types/metadata'

beforeEach(() => {
    genresMock.mockReset()
    genresMock.mockResolvedValue([])
    // Module-scoped cache: without this, one spec's answers serve the next.
    useReleaseGroupGenres().clear()
})

const stubs = {
    Dialog: {
        props: ['visible'],
        template: '<div v-if="visible"><slot /><slot name="footer" /></div>'
    },
    Dropdown: {
        props: ['modelValue', 'options', 'optionLabel', 'optionValue'],
        emits: ['update:modelValue'],
        inheritAttrs: false,
        template: `<select :data-test="$attrs['data-test']" :value="modelValue"
            @change="$emit('update:modelValue', options[$event.target.selectedIndex][optionValue])">
            <option v-for="o in options" :key="o[optionValue]" :value="o[optionValue]">{{ o[optionLabel] }}</option>
        </select>`
    },
    Checkbox: {
        props: ['modelValue', 'binary', 'inputId'],
        template:
            '<input type="checkbox" :id="inputId" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />'
    },
    Button: {
        props: ['label', 'disabled'],
        inheritAttrs: false,
        template:
            '<button :disabled="disabled" :data-test="$attrs[\'data-test\']" @click="$emit(\'click\')">{{ label }}</button>'
    }
}

const mkTrack = (over: Partial<Track> = {}): Track => ({
    path: 'a.mp3',
    name: 'a.mp3',
    title: 'Current Title',
    artists: [],
    album_artists: [],
    album: '',
    genres: [],
    year: 0,
    track_number: 0,
    disc_number: 0,
    disc_subtitle: '',
    compilation: false,
    mb_artist_ids: [],
    mb_album_artist_ids: [],
    mb_recording_id: '',
    mb_release_id: '',
    mb_release_group_id: '',
    ...over
})

const tracks = [mkTrack({ path: '01.mp3' }), mkTrack({ path: '02.mp3' })]

// A row's include checkbox, by the file it belongs to. Queried by id rather than
// by position among all checkboxes: the field-selection row above the table has
// checkboxes too, so an index would silently drift onto one of those.
function includeBox(w: ReturnType<typeof mount>, path: string) {
    return w.find(`#album-include-${path.replace('.', '\\.')}`)
}

const albumA: AlbumOption = {
    release_mbid: 'rel-A',
    release_group_mbid: 'rg-A',
    album: 'Album A',
    year: 1991,
    artists: [{ name: 'Artist', mbid: 'art-1' }],
    track_count: 3,
    disc_count: 1,
    enriched: true,
    matched_count: 1,
    mean_score: 0.9,
    assignments: [
        {
            path: '01.mp3',
            source: 'fingerprint',
            title: 'One',
            recording_mbid: 'rec-1',
            artists: [{ name: 'Artist', mbid: 'art-1' }],
            disc_number: 1,
            track_number: 1,
            score: 0.9
        },
        {
            path: '02.mp3',
            source: 'inferred',
            title: 'Two',
            recording_mbid: 'rec-2',
            artists: [],
            disc_number: 1,
            track_number: 2,
            score: 0
        }
    ],
    tracks: [
        { disc_number: 1, track_number: 1, title: 'One', recording_mbid: 'rec-1', duration_seconds: 180 },
        { disc_number: 1, track_number: 2, title: 'Two', recording_mbid: 'rec-2', duration_seconds: 200 },
        { disc_number: 1, track_number: 3, title: 'Three', recording_mbid: 'rec-3', duration_seconds: 300 }
    ]
}

const albumB: AlbumOption = {
    ...albumA,
    release_mbid: 'rel-B',
    release_group_mbid: 'rg-B',
    album: 'Best Of',
    year: 2005,
    track_count: 20,
    assignments: [
        {
            path: '01.mp3',
            source: 'fingerprint',
            title: 'One (remaster)',
            recording_mbid: 'rec-9',
            artists: [],
            disc_number: 1,
            track_number: 7,
            score: 0.7
        },
        {
            path: '02.mp3',
            source: 'none',
            title: '',
            recording_mbid: '',
            artists: [],
            disc_number: 0,
            track_number: 0,
            score: 0
        }
    ],
    tracks: [
        { disc_number: 1, track_number: 7, title: 'One (remaster)', recording_mbid: 'rec-9', duration_seconds: 180 }
    ]
}

// Multi-disc release: disc 1 track 1 and disc 2 track 1 are distinct positions.
const multiDisc: AlbumOption = {
    release_mbid: 'rel-multi',
    release_group_mbid: 'rg-multi',
    album: 'Double Album',
    year: 2000,
    artists: [{ name: 'Artist', mbid: 'art-1' }],
    track_count: 4,
    disc_count: 2,
    enriched: true,
    matched_count: 2,
    mean_score: 0.8,
    assignments: [
        {
            path: '01.mp3',
            source: 'fingerprint',
            title: 'Disc 1 Track 1',
            recording_mbid: 'rec-d1t1',
            artists: [],
            disc_number: 1,
            track_number: 1,
            score: 0.85
        },
        {
            path: '02.mp3',
            source: 'fingerprint',
            title: 'Disc 2 Track 1',
            recording_mbid: 'rec-d2t1',
            artists: [],
            disc_number: 2,
            track_number: 1,
            score: 0.75
        }
    ],
    tracks: [
        { disc_number: 1, track_number: 1, title: 'Disc 1 Track 1', recording_mbid: 'rec-d1t1', duration_seconds: 200 },
        { disc_number: 1, track_number: 2, title: 'Disc 1 Track 2', recording_mbid: 'rec-d1t2', duration_seconds: 220 },
        { disc_number: 2, track_number: 1, title: 'Disc 2 Track 1', recording_mbid: 'rec-d2t1', duration_seconds: 180 },
        { disc_number: 2, track_number: 2, title: 'Disc 2 Track 2', recording_mbid: 'rec-d2t2', duration_seconds: 240 }
    ]
}

function mountDialog(
    options: AlbumOption[],
    t: Track[] = tracks,
    pathErrors: Array<{ path: string; error: string }> = [],
    loading = false
) {
    return mount(IdentifyAlbumDialog, {
        props: { visible: true, options, tracks: t, pathErrors, loading },
        global: { stubs }
    })
}

describe('IdentifyAlbumDialog track table', () => {
    it('names the staged columns after the fields they write, with no current-value pair', () => {
        const w = mountDialog([albumA])
        const header = w.find('.track-list-header')
        expect(header.exists()).toBe(true)
        const text = header.text()
        expect(text).toContain('#')
        expect(text).toContain('File')
        expect(text).toContain('Title')
        expect(text).toContain('Artist')
        expect(text).toContain('Album')
        expect(text).toContain('Year')
        // The current tags are in the cells' tooltips, not in columns of their
        // own, so nothing is labelled as the "new" value of a pair.
        expect(text).not.toContain('New')
    })

    it('shows the proposed track number in the index column', () => {
        const w = mountDialog([albumA])
        const row = w.find('[data-test="album-row-01.mp3"]')
        expect(row.find('.col-index').text()).toBe('1')
    })

    it('identifies the row by file name, not by its title tag', () => {
        // The fixture's tracks all carry the same title tag ('Current Title'),
        // which is exactly why the column shows the file name: it is the only
        // value that tells two rows apart.
        const w = mountDialog([albumA])
        const file = w.find('[data-test="album-file-01.mp3"]')
        expect(file.text()).toContain('a.mp3')
        // Reads the `name` field, not the path key: a nested file shows its own
        // name in the column and its full path only in the tooltip.
        const nested = mountDialog([albumA], [
            mkTrack({ path: 'CD 1/01.mp3', name: '01 - Song.mp3' }),
            mkTrack({ path: '02.mp3' })
        ])
        expect(nested.find('[data-test="album-file-CD 1/01.mp3"]').text()).toContain(
            '01 - Song.mp3'
        )
        expect(file.text()).not.toContain('Current Title')
        // The proposed title sits in its own column beside it.
        expect(w.find('[data-test="album-title-01.mp3"]').text()).toContain('One')
    })

    it('reports the current title tag in the Title cell tooltip', () => {
        // With the title tag off the table, the tooltip is the only place it
        // appears — so it has to carry it rather than merely flag a change.
        const w = mountDialog([albumA])
        const cell = w.find('[data-test="album-title-01.mp3"]').find('.cell-value')
        expect(cell.attributes('data-pd-tooltip')).toBeDefined()
    })

    it('shows the release album and year on every row, from the chosen option', () => {
        // Both are release-level, so each row stages the same value — but they are
        // shown per row because whether they CHANGE anything is per row.
        const w = mountDialog([albumA])
        expect(w.find('[data-test="album-album-01.mp3"]').text()).toBe('Album A')
        expect(w.find('[data-test="album-album-02.mp3"]').text()).toBe('Album A')
        expect(w.find('[data-test="album-year-01.mp3"]').text()).toBe('1991')
        expect(w.find('[data-test="album-year-02.mp3"]').text()).toBe('1991')
    })

    it('flags album and year per row, so a mixed selection reads correctly', () => {
        // 01.mp3 already carries the release's album and year, 02.mp3 carries
        // neither: the same staged value is a change for one row and not the other.
        const mixed = [
            mkTrack({ path: '01.mp3', album: 'Album A', year: 1991 }),
            mkTrack({ path: '02.mp3' })
        ]
        const w = mountDialog([albumA], mixed)
        expect(w.find('[data-test="album-album-01.mp3"]').classes()).not.toContain('changed')
        expect(w.find('[data-test="album-year-01.mp3"]').classes()).not.toContain('changed')
        expect(w.find('[data-test="album-album-02.mp3"]').classes()).toContain('changed')
        expect(w.find('[data-test="album-year-02.mp3"]').classes()).toContain('changed')
        // And the tooltip only rides the cells that replace something.
        expect(
            w.find('[data-test="album-album-01.mp3"]').find('.cell-value').attributes('data-pd-tooltip')
        ).toBeUndefined()
        expect(
            w.find('[data-test="album-album-02.mp3"]').find('.cell-value').attributes('data-pd-tooltip')
        ).toBeDefined()
    })

    it('stages album and year on a row with no position at all', () => {
        // albumB's 02.mp3 has source 'none', so it stages no title — but the
        // release-level fields still apply to it.
        const w = mountDialog([albumB])
        expect(w.find('[data-test="album-title-02.mp3"]').text()).toContain('unchanged')
        expect(w.find('[data-test="album-album-02.mp3"]').text()).toBe('Best Of')
        expect(w.find('[data-test="album-year-02.mp3"]').text()).toBe('2005')
    })

    it('marks a release with no year as unchanged rather than showing a zero', () => {
        const noYear: AlbumOption = { ...albumA, year: 0 }
        const w = mountDialog([noYear])
        const cell = w.find('[data-test="album-year-01.mp3"]')
        expect(cell.text()).toBe('unchanged')
        expect(cell.classes()).not.toContain('changed')
    })

    it('re-derives album and year when another release is chosen', async () => {
        const w = mountDialog([albumA, albumB])
        expect(w.find('[data-test="album-album-01.mp3"]').text()).toBe('Album A')

        const select = w.find('[data-test="album-select"]')
        ;(select.element as HTMLSelectElement).selectedIndex = 1
        await select.trigger('change')

        expect(w.find('[data-test="album-album-01.mp3"]').text()).toBe('Best Of')
        expect(w.find('[data-test="album-year-01.mp3"]').text()).toBe('2005')
    })

    it('falls back to the release artist when a row carries no credits', () => {
        // albumA's second assignment has no artists of its own, so the target
        // column shows the album artist rather than going blank.
        const w = mountDialog([albumA])
        expect(w.find('[data-test="album-artist-02.mp3"]').text()).toBe('Artist')
    })

    it('flags a changed value and explains what it replaces on hover', () => {
        const w = mountDialog([albumA])
        const title = w.find('[data-test="album-title-01.mp3"]')
        // 'One' replaces 'Current Title', so the cell is marked and the tooltip
        // names the value being replaced — the old value stays reachable without
        // spending a column on it.
        expect(title.classes()).toContain('changed')
        // The directive only marks the element when it was given a non-empty
        // value, so presence here means a tooltip really was attached. (jsdom
        // renders no tooltip text, so the copy itself is covered by the
        // replacesTooltip unit test below.)
        expect(title.find('.cell-value').attributes('data-pd-tooltip')).toBeDefined()

        const artist = w.find('[data-test="album-artist-01.mp3"]')
        expect(artist.classes()).toContain('changed')
        // Same contract as the title: with no current-artist column, the tooltip is
        // the only place the credit being replaced is shown, so it must be attached.
        expect(artist.find('.cell-value').attributes('data-pd-tooltip')).toBeDefined()
    })

    it('does not flag a target equal to the value the file already has', () => {
        // The file is already titled 'One' and credited 'Artist', so identifying
        // it changes nothing and the cell must not shout about it.
        const already = [
            mkTrack({ path: '01.mp3', title: 'One', artists: ['Artist'] }),
            mkTrack({ path: '02.mp3' })
        ]
        const w = mountDialog([albumA], already)
        expect(w.find('[data-test="album-title-01.mp3"]').classes()).not.toContain('changed')
        expect(w.find('[data-test="album-artist-01.mp3"]').classes()).not.toContain('changed')
        // And no tooltip: there is nothing being replaced to explain.
        expect(
            w.find('[data-test="album-title-01.mp3"]').find('.cell-value').attributes('data-pd-tooltip')
        ).toBeUndefined()
    })

    it('marks a row that stages no title as unchanged rather than blank', () => {
        // albumB's second assignment has source 'none', so nothing is proposed.
        const w = mountDialog([albumB])
        const title = w.find('[data-test="album-title-02.mp3"]')
        expect(title.text()).toContain('unchanged')
        expect(title.classes()).not.toContain('changed')
    })

    it('banners each disc only when the release spans more than one', () => {
        const single = mountDialog([albumA])
        expect(single.findAll('.disc-header')).toHaveLength(0)

        const multi = mountDialog([multiDisc])
        const banners = multi.findAll('.disc-header').map((b) => b.text())
        expect(banners).toContain('Disc 1')
        expect(banners).toContain('Disc 2')
    })

    it('marks a conflicting row so it is visible in a long list', () => {
        const clashing: AlbumOption = {
            ...albumA,
            assignments: [
                { ...albumA.assignments[0], path: '01.mp3', disc_number: 1, track_number: 1 },
                { ...albumA.assignments[1], path: '02.mp3', disc_number: 1, track_number: 1 }
            ]
        }
        const w = mountDialog([clashing])
        expect(w.find('[data-test="album-row-01.mp3"]').classes()).toContain('conflicting')
    })

    it('dims a row the user excluded', async () => {
        const w = mountDialog([albumA])
        const row = w.find('[data-test="album-row-01.mp3"]')
        expect(row.classes()).not.toContain('excluded')
        await row.find('input[type="checkbox"]').setValue(false)
        expect(w.find('[data-test="album-row-01.mp3"]').classes()).toContain('excluded')
    })
})

describe('IdentifyAlbumDialog unfilled tracklist positions', () => {
    it('shows a greyed placeholder for a release track the selection lacks', () => {
        // albumA has 3 tracks; the 2 selected files cover 1 and 2, so track 3 is
        // a hole in the selection and must be visible as one.
        const w = mountDialog([albumA])
        const gap = w.find('[data-test="album-gap-1-3"]')
        expect(gap.exists()).toBe(true)
        expect(gap.classes()).toContain('gap-row')
        // It names the album's song but carries no file name and no controls.
        expect(gap.text()).toContain('Three')
        expect(gap.text()).toContain('not in selection')
        expect(gap.find('input[type="checkbox"]').exists()).toBe(false)
        expect(gap.find('select').exists()).toBe(false)
    })

    it('places the placeholder in track order, not appended at the end', () => {
        // Only track 2 is covered, so tracks 1 and 3 are gaps and the rows must
        // read 1, 2, 3 rather than 2, 1, 3.
        const onlyTrackTwo: AlbumOption = {
            ...albumA,
            assignments: [albumA.assignments[1]]
        }
        const w = mountDialog([onlyTrackTwo], [mkTrack({ path: '02.mp3' })])
        const numbers = w
            .findAll('.album-track-row .col-index')
            .map((c) => c.text())
        expect(numbers).toEqual(['1', '2', '3'])
    })

    it('has no placeholders when the selection covers the whole release', () => {
        const full: AlbumOption = {
            ...albumA,
            track_count: 2,
            tracks: albumA.tracks.slice(0, 2)
        }
        const w = mountDialog([full])
        expect(w.findAll('.gap-row')).toHaveLength(0)
    })

    it('brings the placeholder back when its file is unchecked', async () => {
        const w = mountDialog([albumA])
        expect(w.find('[data-test="album-gap-1-1"]').exists()).toBe(false)

        // Unchecking 01.mp3 means track 1 is no longer being filled.
        await w
            .find('[data-test="album-row-01.mp3"]')
            .find('input[type="checkbox"]')
            .setValue(false)
        expect(w.find('[data-test="album-gap-1-1"]').exists()).toBe(true)
    })

    it('shows placeholders per disc on a multi-disc release', () => {
        // multiDisc has 4 positions; the 2 files cover disc 1 track 1 and disc 2
        // track 1, leaving one gap on each disc.
        const w = mountDialog([multiDisc])
        expect(w.find('[data-test="album-gap-1-2"]').exists()).toBe(true)
        expect(w.find('[data-test="album-gap-2-2"]').exists()).toBe(true)
    })

    it('does not show placeholders for an un-enriched option', () => {
        // No tracklist was fetched, so the release's other songs are unknown —
        // inventing placeholders would be a guess.
        const unenriched: AlbumOption = {
            ...albumA,
            enriched: false,
            track_count: 0,
            tracks: []
        }
        const w = mountDialog([unenriched])
        expect(w.findAll('.gap-row')).toHaveLength(0)
    })
})

describe('IdentifyAlbumDialog loading state', () => {
    it('shows progress instead of results while the request is in flight', () => {
        const w = mountDialog([], tracks, [], true)
        expect(w.find('[data-test="album-loading"]').exists()).toBe(true)
        expect(w.text()).toContain('Identifying 2 songs')
        // Nothing to review yet: no album picker, no rows, no staging button,
        // and crucially not the "nothing matched" copy, which would be a lie.
        expect(w.find('[data-test="album-select"]').exists()).toBe(false)
        expect(w.find('[data-test="album-apply"]').exists()).toBe(false)
        expect(w.find('[data-test="album-empty"]').exists()).toBe(false)
    })

    it('swaps the progress state for the results once options arrive', async () => {
        const w = mountDialog([], tracks, [], true)
        expect(w.find('[data-test="album-loading"]').exists()).toBe(true)

        await w.setProps({ loading: false, options: [albumA] })
        expect(w.find('[data-test="album-loading"]').exists()).toBe(false)
        expect(w.find('[data-test="album-select"]').exists()).toBe(true)
        expect(w.find('[data-test="album-apply"]').exists()).toBe(true)
    })

    it('hides the loading state even when the request reported per-file errors', async () => {
        const w = mountDialog([], tracks, [{ path: 'x.mp3', error: 'could not be read' }], false)
        expect(w.find('[data-test="album-loading"]').exists()).toBe(false)
        expect(w.find('[data-test="album-path-errors"]').exists()).toBe(true)
    })

    it('emits cancel alongside close so the parent can abort the request', async () => {
        const w = mountDialog([], tracks, [], true)
        await w.find('[data-test="album-cancel"]').trigger('click')
        expect(w.emitted('cancel')).toHaveLength(1)
        expect(w.emitted('update:visible')![0]).toEqual([false])
    })

    it('emits cancel when the dialog is dismissed without the Cancel button', async () => {
        // The header X and Escape both come through the Dialog's update:visible,
        // and they must abort too — otherwise the request runs on invisibly.
        const w = mountDialog([], tracks, [], true)
        await w.findComponent(stubs.Dialog).vm.$emit('update:visible', false)
        expect(w.emitted('cancel')).toHaveLength(1)
    })

    it('still offers Cancel after results arrive', async () => {
        const w = mountDialog([albumA])
        expect(w.find('[data-test="album-cancel"]').exists()).toBe(true)
        await w.find('[data-test="album-cancel"]').trigger('click')
        // No request is in flight by then; the parent's abort is a no-op.
        expect(w.emitted('update:visible')![0]).toEqual([false])
    })
})

describe('IdentifyAlbumDialog', () => {
    it('preselects the first option and stages every song on apply', async () => {
        const w = mountDialog([albumA, albumB])
        expect(w.text()).toContain('Album A')
        expect(w.text()).toContain('One')
        expect(w.text()).toContain('Two')

        await w.find('[data-test="album-apply"]').trigger('click')
        const picks = w.emitted('apply')![0][0] as any[]
        expect(picks).toHaveLength(2)
        expect(picks[0].option.release_mbid).toBe('rel-A')
        expect(picks[0].assignment.track_number).toBe(1)
        expect(picks[1].assignment.track_number).toBe(2)
    })

    it('marks each row with its assignment source', () => {
        const w = mountDialog([albumA])
        expect(w.find('[data-test="album-badge-01.mp3"]').text()).toContain('fingerprint')
        expect(w.find('[data-test="album-badge-02.mp3"]').text()).toContain('inferred')
    })

    it('re-derives the rows when another album is chosen', async () => {
        const w = mountDialog([albumA, albumB])
        const select = w.find('[data-test="album-select"]')
        ;(select.element as HTMLSelectElement).selectedIndex = 1
        await select.trigger('change')

        expect(w.text()).toContain('One (remaster)')
        expect(w.find('[data-test="album-badge-02.mp3"]').text()).toContain('none')

        await w.find('[data-test="album-apply"]').trigger('click')
        const picks = w.emitted('apply')![0][0] as any[]
        expect(picks[0].option.release_mbid).toBe('rel-B')
        // A none-source row still stages the album fields, with no position.
        const second = picks.find((p: any) => p.path === '02.mp3')
        expect(second.assignment).toBeNull()
    })

    it('excludes an unchecked song from the picks', async () => {
        const w = mountDialog([albumA])
        await includeBox(w, '01.mp3').setValue(false)
        await w.find('[data-test="album-apply"]').trigger('click')
        const picks = w.emitted('apply')![0][0] as any[]
        expect(picks.map((p: any) => p.path)).toEqual(['02.mp3'])
    })

    it('labels the apply button with the included count', async () => {
        const w = mountDialog([albumA])
        expect(w.find('[data-test="album-apply"]').text()).toBe('Stage 2 songs')
        await includeBox(w, '01.mp3').setValue(false)
        expect(w.find('[data-test="album-apply"]').text()).toBe('Stage 1 song')
    })

    it('offers only free tracklist slots for re-pointing and frees the old one', async () => {
        const w = mountDialog([albumA])
        const rowSelect = w.find('[data-test="album-slot-02.mp3"]')
        // Row 02.mp3 is on slot 2. It should show slot 2 (mine) and slot 3 (free),
        // but not slot 1 (taken by 01.mp3).
        expect(rowSelect.text()).toContain('3')
        expect(rowSelect.text()).toContain('Two') // mine
        expect(rowSelect.text()).not.toContain('One') // taken by another row
        // The taken positions (except mine) are genuinely absent from the choices.
        const options = rowSelect.findAll('option')
        const values = options.map(o => o.element.value)
        expect(values).toContain('1-2') // mine — I can keep it
        expect(values).toContain('1-3') // slot 3 is free
        expect(values).not.toContain('1-1') // slot 1 is taken by 01.mp3
    })

    it('disables apply when two rows resolve to the same position', async () => {
        const clashing: AlbumOption = {
            ...albumA,
            assignments: [
                { ...albumA.assignments[0], path: '01.mp3', track_number: 1 },
                { ...albumA.assignments[1], path: '02.mp3', track_number: 1 }
            ]
        }
        const w = mountDialog([clashing])
        expect(w.find('[data-test="album-conflict"]').exists()).toBe(true)
        expect(w.find('[data-test="album-apply"]').attributes('disabled')).toBeDefined()
    })

    // An un-enriched option carries no artist credits, and a Go nil slice
    // marshals to JSON null, so `artists` can arrive null despite the type. The
    // album label is rendered for every option during setup, so an unguarded
    // .map() there takes the entire dialog down instead of degrading one label.
    it('renders an option whose artist list arrived as null', () => {
        const noArtists = { ...albumA, artists: null } as unknown as AlbumOption
        const w = mountDialog([noArtists])
        expect(w.find('[data-test="album-select"]').exists()).toBe(true)
        expect(w.text()).toContain('Album A')
        expect(w.find('[data-test="album-apply"]').exists()).toBe(true)
    })

    it('shows an empty state when nothing matched', () => {
        const w = mountDialog([])
        expect(w.find('[data-test="album-empty"]').exists()).toBe(true)
        expect(w.find('[data-test="album-apply"]').exists()).toBe(false)
    })

    it('shows a per-file error on its row', () => {
        const withError: AlbumOption = {
            ...albumA,
            assignments: [
                albumA.assignments[0],
                {
                    ...albumA.assignments[1],
                    source: 'none',
                    title: '',
                    track_number: 0,
                    error: 'fingerprint failed'
                }
            ]
        }
        const w = mountDialog([withError])
        expect(w.find('[data-test="album-row-02.mp3"]').text()).toContain('fingerprint failed')
    })

    it('shows path errors even when no options exist', () => {
        // The reasons the server actually sends: short, user-facing sentences.
        const pathErrs = [
            { path: 'bad.mp3', error: 'could not be fingerprinted' },
            { path: '../outside.mp3', error: 'is outside the library' }
        ]
        const w = mountDialog([], [], pathErrs)
        expect(w.find('[data-test="album-path-errors"]').exists()).toBe(true)
        expect(w.text()).toContain('bad.mp3')
        expect(w.text()).toContain('could not be fingerprinted')
        expect(w.text()).toContain('../outside.mp3')
        expect(w.text()).toContain('is outside the library')
    })

    // errors[] carries library-resolution refusals too, which were never
    // fingerprinted, so the header must not claim fingerprinting for all of them.
    it('labels the error block for both refusals and fingerprint failures', () => {
        const w = mountDialog([], [], [{ path: '../outside.mp3', error: 'is outside the library' }])
        const header = w.find('[data-test="album-path-errors"]').text()
        expect(header).not.toContain('could not be fingerprinted:')
        expect(header).toContain('were not identified')
    })

    // Multi-disc tests: a position is a (disc, track) pair, not a bare track number.
    it('multi-disc: both same-numbered slots from different discs are offered', () => {
        const w = mountDialog([multiDisc])
        const rowSelect = w.find('[data-test="album-slot-01.mp3"]')
        const options = rowSelect.findAll('option')
        const values = options.map(o => o.element.value)
        // Disc 1 track 1 is taken by this row (01.mp3), so it IS shown (mine).
        // Disc 2 track 1 is taken by 02.mp3, so it's NOT shown.
        // Both disc 1 track 2 and disc 2 track 2 are free.
        expect(values).toContain('1-1') // mine — I can keep it
        expect(values).toContain('1-2') // free on disc 1
        expect(values).toContain('2-2') // free on disc 2
        expect(values).not.toContain('2-1') // taken by 02.mp3, not mine
    })

    it('multi-disc: re-pointing to disc 2 track 1 stages disc 2 metadata', async () => {
        const w = mountDialog([multiDisc])
        // Uncheck 02.mp3 first so disc 2 track 1 becomes available for 01.mp3.
        await includeBox(w, '02.mp3').setValue(false)

        // Now 01.mp3's dropdown offers disc 2 track 1 (it was taken by 02.mp3 before).
        const rowSelect = w.find('[data-test="album-slot-01.mp3"]')
        const options = rowSelect.findAll('option')
        const targetIndex = options.findIndex(o => o.element.value === '2-1')
        expect(targetIndex).toBeGreaterThan(-1) // slot should exist now
        ;(rowSelect.element as HTMLSelectElement).selectedIndex = targetIndex
        await rowSelect.trigger('change')

        await w.find('[data-test="album-apply"]').trigger('click')
        const picks = w.emitted('apply')![0][0] as any[]
        expect(picks).toHaveLength(1)
        expect(picks[0].path).toBe('01.mp3')
        expect(picks[0].assignment.disc_number).toBe(2)
        expect(picks[0].assignment.track_number).toBe(1)
        expect(picks[0].assignment.title).toBe('Disc 2 Track 1')
        expect(picks[0].assignment.recording_mbid).toBe('rec-d2t1')
    })

    it('multi-disc: disc 1 track 1 and disc 2 track 1 are not a conflict', () => {
        const w = mountDialog([multiDisc])
        // Both rows have track_number 1 but on different discs — no conflict.
        expect(w.find('[data-test="album-conflict"]').exists()).toBe(false)
        expect(w.find('[data-test="album-apply"]').attributes('disabled')).toBeUndefined()
    })

    it('multi-disc: unchecking a row frees its position', async () => {
        const w = mountDialog([multiDisc])
        // Initially, disc 1 track 1 is taken by 01.mp3, so 02.mp3's dropdown does not offer it.
        const row2Select = w.find('[data-test="album-slot-02.mp3"]')
        let options = row2Select.findAll('option')
        let values = options.map(o => o.element.value)
        expect(values).not.toContain('1-1')

        // Uncheck 01.mp3 to free disc 1 track 1.
        await includeBox(w, '01.mp3').setValue(false)

        // Now 02.mp3's dropdown offers disc 1 track 1.
        options = row2Select.findAll('option')
        values = options.map(o => o.element.value)
        expect(values).toContain('1-1')
    })
})

describe('IdentifyAlbumDialog field selection', () => {
    it('offers a checkbox per stageable field, all selected by default', async () => {
        const w = mountDialog([albumA])
        expect(w.find('[data-test="album-fields"]').exists()).toBe(true)
        for (const field of IDENTIFY_FIELDS) {
            const box = w.find(`[data-test="album-field-${field.id}"]`)
            expect(box.exists()).toBe(true)
            expect((box.element as HTMLInputElement).checked).toBe(true)
        }

        await w.find('[data-test="album-apply"]').trigger('click')
        expect(w.emitted('apply')![0][1]).toEqual([...ALL_IDENTIFY_FIELD_IDS])
    })

    it('hides the field row while the request is still in flight', () => {
        const w = mountDialog([], tracks, [], true)
        expect(w.find('[data-test="album-fields"]').exists()).toBe(false)
    })

    it('emits only the fields left checked', async () => {
        // The album case this exists for: a rip whose titles are already right,
        // where only the release-level tags should be taken from the match.
        const w = mountDialog([albumA])
        await w.find('[data-test="album-field-title"]').setValue(false)
        await w.find('[data-test="album-field-track_number"]').setValue(false)

        await w.find('[data-test="album-apply"]').trigger('click')
        const fields = w.emitted('apply')![0][1] as string[]
        expect(fields).not.toContain('title')
        expect(fields).not.toContain('track_number')
        expect(fields).toContain('album')
        // The picks are unaffected: narrowing is about which values get staged,
        // not which songs were included.
        expect((w.emitted('apply')![0][0] as any[]).map((p: any) => p.path)).toEqual([
            '01.mp3',
            '02.mp3'
        ])
    })

    it('None clears every field and blocks apply until something is picked', async () => {
        const w = mountDialog([albumA])
        await w.find('[data-test="album-fields-none"]').trigger('click')
        expect(w.find('[data-test="album-apply"]').attributes('disabled')).toBeDefined()

        await w.find('[data-test="album-field-album"]').setValue(true)
        expect(w.find('[data-test="album-apply"]').attributes('disabled')).toBeUndefined()
        await w.find('[data-test="album-apply"]').trigger('click')
        expect(w.emitted('apply')![0][1]).toEqual(['album'])
    })

    it('All restores the full selection after narrowing it', async () => {
        const w = mountDialog([albumA])
        await w.find('[data-test="album-fields-none"]').trigger('click')
        await w.find('[data-test="album-fields-all"]').trigger('click')
        await w.find('[data-test="album-apply"]').trigger('click')
        expect(w.emitted('apply')![0][1]).toEqual([...ALL_IDENTIFY_FIELD_IDS])
    })

    it('emits the fields in registry order regardless of click order', async () => {
        const w = mountDialog([albumA])
        await w.find('[data-test="album-fields-none"]').trigger('click')
        await w.find('[data-test="album-field-year"]').setValue(true)
        await w.find('[data-test="album-field-album"]').setValue(true)
        await w.find('[data-test="album-apply"]').trigger('click')
        expect(w.emitted('apply')![0][1]).toEqual(['album', 'year'])
    })

    it('keeps the selection when another album is chosen', async () => {
        // Switching releases resets the per-row positions, but which FIELDS to
        // stage is a preference about the tags, not about the tracklist.
        const w = mountDialog([albumA, albumB])
        await w.find('[data-test="album-fields-none"]').trigger('click')
        await w.find('[data-test="album-field-album"]').setValue(true)

        const select = w.find('[data-test="album-select"]')
        ;(select.element as HTMLSelectElement).selectedIndex = 1
        await select.trigger('change')

        await w.find('[data-test="album-apply"]').trigger('click')
        expect(w.emitted('apply')![0][1]).toEqual(['album'])
    })
})

describe('IdentifyAlbumDialog genres', () => {
    it('looks up the genres of the preselected option release group', async () => {
        genresMock.mockResolvedValue(['Grunge', 'Alternative Rock'])
        mountDialog([albumA])
        await flushPromises()
        expect(genresMock).toHaveBeenCalledWith('rg-A')
    })

    it('shows the genres it will stage', async () => {
        genresMock.mockResolvedValue(['Grunge', 'Alternative Rock'])
        const w = mountDialog([albumA])
        await flushPromises()
        expect(w.find('[data-test="album-genres"]').text()).toContain('Grunge')
        expect(w.find('[data-test="album-genres"]').text()).toContain('Alternative Rock')
    })

    it('carries the genres on every pick', async () => {
        genresMock.mockResolvedValue(['Grunge'])
        const w = mountDialog([albumA])
        await flushPromises()
        await w.find('[data-test="album-apply"]').trigger('click')
        const picks = w.emitted('apply')![0][0] as any[]
        expect(picks).toHaveLength(2)
        for (const p of picks) expect(p.genres).toEqual(['Grunge'])
    })

    it('re-looks-up when the user picks another album', async () => {
        genresMock.mockResolvedValueOnce(['Grunge'])
        genresMock.mockResolvedValueOnce(['Compilation Rock'])
        const w = mountDialog([albumA, albumB])
        await flushPromises()

        const select = w.find('[data-test="album-select"]')
        ;(select.element as HTMLSelectElement).selectedIndex = 1
        await select.trigger('change')
        await flushPromises()

        expect(genresMock).toHaveBeenCalledWith('rg-B')
        await w.find('[data-test="album-apply"]').trigger('click')
        const picks = w.emitted('apply')![0][0] as any[]
        expect(picks[0].genres).toEqual(['Compilation Rock'])
    })

    it('does not re-request a release group it already looked up', async () => {
        // MusicBrainz is throttled to one request per second, so switching back
        // and forth between two options must not queue a request each time.
        genresMock.mockResolvedValue(['Grunge'])
        const w = mountDialog([albumA, albumB])
        await flushPromises()
        const select = w.find('[data-test="album-select"]')
        ;(select.element as HTMLSelectElement).selectedIndex = 1
        await select.trigger('change')
        await flushPromises()
        ;(select.element as HTMLSelectElement).selectedIndex = 0
        await select.trigger('change')
        await flushPromises()

        expect(genresMock).toHaveBeenCalledTimes(2)
        expect(genresMock).toHaveBeenNthCalledWith(1, 'rg-A')
        expect(genresMock).toHaveBeenNthCalledWith(2, 'rg-B')
    })

    it('stages no genres and hides the row when the lookup fails', async () => {
        // A failed genre lookup must not block the apply: the rest of the match
        // is still worth staging.
        genresMock.mockRejectedValue(new Error('rate limited'))
        const w = mountDialog([albumA])
        await flushPromises()
        expect(w.find('[data-test="album-genres"]').exists()).toBe(false)
        await w.find('[data-test="album-apply"]').trigger('click')
        expect((w.emitted('apply')![0][0] as any[])[0].genres).toEqual([])
    })

    it('hides the row when MusicBrainz holds no genres for the group', async () => {
        genresMock.mockResolvedValue([])
        const w = mountDialog([albumA])
        await flushPromises()
        expect(w.find('[data-test="album-genres"]').exists()).toBe(false)
    })

    it('makes no request while the identify run is still in flight', async () => {
        // There is no option to look genres up for yet.
        mountDialog([], tracks, [], true)
        await flushPromises()
        expect(genresMock).not.toHaveBeenCalled()
    })

    it('does not show a stale answer after the user switched albums', async () => {
        // rg-A resolves AFTER the user has already moved to rg-B; the late answer
        // belongs to an option that is no longer selected.
        let resolveA: (v: string[]) => void = () => {}
        genresMock.mockImplementationOnce(
            () => new Promise<string[]>((res) => (resolveA = res))
        )
        genresMock.mockResolvedValueOnce(['Compilation Rock'])

        const w = mountDialog([albumA, albumB])
        const select = w.find('[data-test="album-select"]')
        ;(select.element as HTMLSelectElement).selectedIndex = 1
        await select.trigger('change')
        await flushPromises()

        resolveA(['Grunge'])
        await flushPromises()

        expect(w.find('[data-test="album-genres"]').text()).toContain('Compilation Rock')
        expect(w.find('[data-test="album-genres"]').text()).not.toContain('Grunge')
    })
})

describe('IdentifyAlbumDialog re-identify', () => {
    it('offers a re-identify action once options are on screen', async () => {
        const w = mountDialog([albumA])
        await w.find('[data-test="album-reidentify"]').trigger('click')
        expect(w.emitted('reidentify')).toHaveLength(1)
    })

    it('offers nothing to re-run while the request is still in flight', () => {
        const w = mountDialog([], tracks, [], true)
        expect(w.find('[data-test="album-reidentify"]').exists()).toBe(false)
    })

    it('offers a re-identify action even when nothing matched', async () => {
        // A run that found no release is exactly the one worth retrying — the
        // upstream lookup may simply have been rate-limited.
        const w = mountDialog([])
        await w.find('[data-test="album-reidentify"]').trigger('click')
        expect(w.emitted('reidentify')).toHaveLength(1)
    })

    it('does not close the dialog: the fresh run repopulates it in place', async () => {
        const w = mountDialog([albumA])
        await w.find('[data-test="album-reidentify"]').trigger('click')
        expect(w.emitted('update:visible')).toBeUndefined()
        expect(w.emitted('cancel')).toBeUndefined()
    })
})
