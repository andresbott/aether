import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import IdentifyAlbumDialog from '@/views/settings/metadata-editor/IdentifyAlbumDialog.vue'
import type { AlbumOption, Track } from '@/types/metadata'

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

function mountDialog(options: AlbumOption[], t: Track[] = tracks, pathErrors: Array<{ path: string; error: string }> = []) {
    return mount(IdentifyAlbumDialog, {
        props: { visible: true, options, tracks: t, pathErrors },
        global: { stubs }
    })
}

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
        const boxes = w.findAll('input[type="checkbox"]')
        await boxes[0].setValue(false)
        await w.find('[data-test="album-apply"]').trigger('click')
        const picks = w.emitted('apply')![0][0] as any[]
        expect(picks.map((p: any) => p.path)).toEqual(['02.mp3'])
    })

    it('labels the apply button with the included count', async () => {
        const w = mountDialog([albumA])
        expect(w.find('[data-test="album-apply"]').text()).toBe('Stage 2 songs')
        await w.findAll('input[type="checkbox"]')[0].setValue(false)
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
        const pathErrs = [
            { path: 'bad.mp3', error: 'fpcalc not found' },
            { path: 'corrupt.mp3', error: 'invalid audio format' }
        ]
        const w = mountDialog([], [], pathErrs)
        expect(w.find('[data-test="album-path-errors"]').exists()).toBe(true)
        expect(w.text()).toContain('bad.mp3')
        expect(w.text()).toContain('fpcalc not found')
        expect(w.text()).toContain('corrupt.mp3')
        expect(w.text()).toContain('invalid audio format')
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
        const boxes = w.findAll('input[type="checkbox"]')
        await boxes[1].setValue(false)

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
        const boxes = w.findAll('input[type="checkbox"]')
        await boxes[0].setValue(false)

        // Now 02.mp3's dropdown offers disc 1 track 1.
        options = row2Select.findAll('option')
        values = options.map(o => o.element.value)
        expect(values).toContain('1-1')
    })
})
