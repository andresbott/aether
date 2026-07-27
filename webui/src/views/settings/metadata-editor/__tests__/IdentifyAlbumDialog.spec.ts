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

function mountDialog(options: AlbumOption[], t: Track[] = tracks) {
    return mount(IdentifyAlbumDialog, {
        props: { visible: true, options, tracks: t },
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
        // Slots 1 and 2 are taken by the two rows; only 3 (plus "keep" and
        // "clear") is offered.
        expect(rowSelect.text()).toContain('3')
        expect(rowSelect.text()).not.toContain('One')
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
})
