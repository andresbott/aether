import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AlbumCandidatePicker from '@/views/settings/metadata-editor/AlbumCandidatePicker.vue'
import type { AlbumOption, Track } from '@/types/metadata'

const stubs = {
    Dialog: {
        props: ['visible'],
        template: '<div v-if="visible"><slot /><slot name="footer" /></div>'
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
    artists: [{ name: 'Nirvana', mbid: 'art-1' }],
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
            artists: [{ name: 'Nirvana', mbid: 'art-1' }],
            disc_number: 1,
            track_number: 1,
            score: 0.9
        }
    ],
    tracks: [
        { disc_number: 1, track_number: 1, title: 'One', recording_mbid: 'rec-1', duration_seconds: 180 }
    ]
}

const albumB: AlbumOption = {
    ...albumA,
    release_mbid: 'rel-B',
    release_group_mbid: 'rg-B',
    album: 'Best Of',
    year: 2005,
    artists: [{ name: 'Various Artists', mbid: 'art-2' }],
    track_count: 20,
    disc_count: 2,
    enriched: true,
    matched_count: 2,
    mean_score: 0.75
}

function mountPicker(over: {
    options?: AlbumOption[]
    selectedMbid?: string
    t?: Track[]
} = {}) {
    return mount(AlbumCandidatePicker, {
        props: {
            visible: true,
            options: over.options ?? [albumA, albumB],
            selectedMbid: over.selectedMbid ?? 'rel-A',
            tracks: over.t ?? tracks
        },
        global: { stubs }
    })
}

describe('AlbumCandidatePicker', () => {
    it('renders one row per candidate with album, artist and year', () => {
        const w = mountPicker()
        expect(w.findAll('[data-test^="candidate-row-"]')).toHaveLength(2)
        const rowA = w.find('[data-test="candidate-row-rel-A"]')
        expect(rowA.text()).toContain('Album A')
        expect(rowA.text()).toContain('Nirvana')
        expect(rowA.text()).toContain('1991')
    })

    it('shows coverage over the selection and the match confidence', () => {
        const w = mountPicker()
        const rowA = w.find('[data-test="candidate-row-rel-A"]')
        // 1 of the 2 selected files matched this release, at a 0.9 mean score.
        expect(rowA.text()).toContain('1 / 2')
        expect(rowA.text()).toContain('90%')
    })

    it('shows the release size and flags an un-enriched option', () => {
        const unenriched: AlbumOption = { ...albumA, enriched: false, track_count: 0, disc_count: 0 }
        const w = mountPicker({ options: [albumB, unenriched] })
        // Enriched: track and disc counts. Un-enriched: no tracklist was fetched.
        expect(w.find('[data-test="candidate-row-rel-B"]').text()).toContain('20 tracks')
        expect(w.find('[data-test="candidate-row-rel-A"]').text()).toContain('track list unavailable')
    })

    it('summarizes the tag changes each option would make', () => {
        // Two bare files against albumA, which places only 01.mp3: it rewrites
        // that one title, and the release-level album+year on both rows.
        const w = mountPicker({ options: [albumA] })
        const changes = w.find('[data-test="candidate-changes-rel-A"]')
        expect(changes.text()).toContain('1 title')
        expect(changes.text()).toContain('2 albums')
    })

    it('preselects the row matching selectedMbid', () => {
        const w = mountPicker({ selectedMbid: 'rel-B' })
        expect(
            (w.find('[data-test="candidate-radio-rel-B"]').element as HTMLInputElement).checked
        ).toBe(true)
        expect(w.find('[data-test="candidate-row-rel-B"]').classes()).toContain('selected')
        expect(w.find('[data-test="candidate-row-rel-A"]').classes()).not.toContain('selected')
    })

    it('confirms the newly picked release and closes', async () => {
        const w = mountPicker({ selectedMbid: 'rel-A' })
        await w.find('[data-test="candidate-radio-rel-B"]').setValue(true)
        await w.find('[data-test="candidate-confirm"]').trigger('click')
        expect(w.emitted('select')).toEqual([['rel-B']])
        expect(w.emitted('update:visible')![0]).toEqual([false])
    })

    it('keeps the preselection when confirmed without changing it', async () => {
        const w = mountPicker({ selectedMbid: 'rel-B' })
        await w.find('[data-test="candidate-confirm"]').trigger('click')
        expect(w.emitted('select')).toEqual([['rel-B']])
    })

    it('cancel closes without choosing a release', async () => {
        const w = mountPicker()
        await w.find('[data-test="candidate-cancel"]').trigger('click')
        expect(w.emitted('update:visible')![0]).toEqual([false])
        expect(w.emitted('select')).toBeUndefined()
    })

    it('links each candidate to its MusicBrainz release page', () => {
        const w = mountPicker()
        expect(w.find('[data-test="candidate-mb-rel-A"]').attributes('href')).toBe(
            'https://musicbrainz.org/release/rel-A'
        )
    })

    it('renders a candidate whose artist list arrived as null', () => {
        const noArtists = { ...albumA, artists: null } as unknown as AlbumOption
        const w = mountPicker({ options: [noArtists, albumB] })
        expect(w.find('[data-test="candidate-row-rel-A"]').exists()).toBe(true)
        expect(w.find('[data-test="candidate-row-rel-A"]').text()).toContain('Album A')
    })
})
