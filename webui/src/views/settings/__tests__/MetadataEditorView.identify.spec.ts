import { describe, it, expect, vi } from 'vitest'
import { useEditSession, candidateToOverlay } from '@/composables/useEditSession'
import { pickOverlayFields } from '@/lib/identifyFields'
import type { IdentifyPick, Track } from '@/types/metadata'

// Stub the side-effecting pieces that useEditSession depends on
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

// Helper to create a test track
const mkTrack = (path: string, title: string, album: string): Track => ({
    path,
    name: path,
    title,
    artists: ['Test Artist'],
    album_artists: [],
    album,
    genres: [],
    year: 2020,
    track_number: 1,
    disc_number: 1,
    disc_subtitle: '',
    compilation: false,
    mb_artist_ids: [],
    mb_album_artist_ids: [],
    mb_recording_id: '',
    mb_release_id: '',
    mb_release_group_id: ''
})

describe('MetadataEditorView identify', () => {
    it('onIdentifyApply stages the picked fields onto the session without re-selecting', () => {
        // Arrange: create a session with two tracks
        const tracks = [
            mkTrack('track1.mp3', 'Original Title 1', 'Original Album'),
            mkTrack('track2.mp3', 'Original Title 2', 'Original Album')
        ]
        const session = useEditSession(
            () => tracks,
            () => 1
        )

        expect(session.overlays.value.size).toBe(0)

        // Arrange: create identify picks that change title and album
        const picks: IdentifyPick[] = [
            {
                path: 'track1.mp3',
                candidate: {
                    score: 0.95,
                    recording_mbid: 'rec1',
                    title: 'Identified Title 1',
                    artists: [{ name: 'Identified Artist 1', mbid: 'artist1' }],
                    releases: [
                        {
                            release_mbid: 'rel1',
                            release_group_mbid: 'rg1',
                            album: 'Identified Album',
                            year: 2021,
                            track_number: 1,
                            disc_number: 1
                        }
                    ]
                },
                release: {
                    release_mbid: 'rel1',
                    release_group_mbid: 'rg1',
                    album: 'Identified Album',
                    year: 2021,
                    track_number: 1,
                    disc_number: 1
                },
                genres: []
            },
            {
                path: 'track2.mp3',
                candidate: {
                    score: 0.93,
                    recording_mbid: 'rec2',
                    title: 'Identified Title 2',
                    artists: [{ name: 'Identified Artist 2', mbid: 'artist2' }],
                    releases: [
                        {
                            release_mbid: 'rel1',
                            release_group_mbid: 'rg1',
                            album: 'Identified Album',
                            year: 2021,
                            track_number: 2,
                            disc_number: 1
                        }
                    ]
                },
                release: {
                    release_mbid: 'rel1',
                    release_group_mbid: 'rg1',
                    album: 'Identified Album',
                    year: 2021,
                    track_number: 2,
                    disc_number: 1
                },
                genres: []
            }
        ]

        const fields = ['title', 'album'] as const

        // Act: mimic onIdentifyApply's logic - convert picks to overlays and stage them
        const entries = new Map(
            picks.map((p) => [
                p.path,
                pickOverlayFields(candidateToOverlay(p.candidate, p.release, p.genres), fields)
            ])
        )
        session.stageOverlays(entries)

        // Assert: the session now has the overlays staged with the expected values
        expect(session.overlays.value.size).toBe(2)
        expect(session.overlays.value.get('track1.mp3')).toEqual({
            title: 'Identified Title 1',
            album: 'Identified Album'
        })
        expect(session.overlays.value.get('track2.mp3')).toEqual({
            title: 'Identified Title 2',
            album: 'Identified Album'
        })

        // Assert: the session recognizes it has changes
        expect(session.hasStagedChanges.value).toBe(true)
    })
})
