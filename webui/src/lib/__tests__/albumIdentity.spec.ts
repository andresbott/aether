import { describe, it, expect } from 'vitest'
import { albumKey, dirOf, selectionAlbumKey, selectionDirs } from '@/lib/albumIdentity'
import type { Track } from '@/types/metadata'

const mkTrack = (over: Partial<Track> = {}): Track => ({
    path: 'album/a.mp3',
    name: 'a.mp3',
    title: '',
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

describe('albumKey', () => {
    it('groups the discs of a multi-folder release under one key', () => {
        const cd1 = mkTrack({ path: 'Release/CD 1/01.mp3', album: 'Sensaciones De Locura' })
        const cd2 = mkTrack({ path: 'Release/CD 2/01.mp3', album: 'Sensaciones De Locura' })
        expect(albumKey(cd1)).toBe(albumKey(cd2))
    })

    it('ignores case and surrounding whitespace in the album name', () => {
        expect(albumKey(mkTrack({ album: '  The Album ' }))).toBe(
            albumKey(mkTrack({ album: 'the album' }))
        )
    })

    it('separates different albums that share a folder', () => {
        const a = mkTrack({ path: 'mixed/01.mp3', album: 'One' })
        const b = mkTrack({ path: 'mixed/02.mp3', album: 'Two' })
        expect(albumKey(a)).not.toBe(albumKey(b))
    })

    it('separates same-named albums by album artist', () => {
        const a = mkTrack({ album: 'Greatest Hits', album_artists: ['Queen'] })
        const b = mkTrack({ album: 'Greatest Hits', album_artists: ['ABBA'] })
        expect(albumKey(a)).not.toBe(albumKey(b))
    })

    it('separates same-named albums by MusicBrainz release id', () => {
        const a = mkTrack({ album: 'Debut', mb_release_id: 'rel-1' })
        const b = mkTrack({ album: 'Debut', mb_release_id: 'rel-2' })
        expect(albumKey(a)).not.toBe(albumKey(b))
    })

    it('falls back to the directory when the album tag is empty', () => {
        // Untagged files must not collapse into one pseudo-album: two folders
        // of unknown albums stay separate, one folder stays together.
        const a = mkTrack({ path: 'Folder A/01.mp3', album: '' })
        const b = mkTrack({ path: 'Folder B/01.mp3', album: '' })
        const c = mkTrack({ path: 'Folder A/02.mp3', album: '   ' })
        expect(albumKey(a)).not.toBe(albumKey(b))
        expect(albumKey(a)).toBe(albumKey(c))
    })
})

describe('selectionAlbumKey', () => {
    it('returns the shared key for the discs of one release', () => {
        const tracks = [
            mkTrack({ path: 'Release/CD 1/01.mp3', album: 'Sensaciones' }),
            mkTrack({ path: 'Release/CD 2/01.mp3', album: 'Sensaciones' })
        ]
        expect(selectionAlbumKey(tracks)).toBe(albumKey(tracks[0]))
    })

    it('falls back to the directory when one folder holds several albums', () => {
        // A compilation folder whose tracks each name a different album is still
        // one folder, so its folder art stays editable.
        const tracks = [
            mkTrack({ path: 'mixed/01.mp3', album: 'One' }),
            mkTrack({ path: 'mixed/02.mp3', album: 'Two' })
        ]
        expect(selectionAlbumKey(tracks)).toBe(albumKey(mkTrack({ path: 'mixed/01.mp3' })))
    })

    it('returns null when the selection spans both albums and folders', () => {
        expect(
            selectionAlbumKey([
                mkTrack({ path: 'Album A/01.mp3', album: 'A' }),
                mkTrack({ path: 'Album B/01.mp3', album: 'B' })
            ])
        ).toBeNull()
    })

    it('returns null for an empty selection', () => {
        expect(selectionAlbumKey([])).toBeNull()
    })
})

describe('dirOf', () => {
    it('returns the parent directory, or the empty string at the root', () => {
        expect(dirOf('Artist/Album/01.mp3')).toBe('Artist/Album')
        expect(dirOf('01.mp3')).toBe('')
    })
})

describe('selectionDirs', () => {
    it('returns the distinct directories in stable sorted order', () => {
        const dirs = selectionDirs([
            mkTrack({ path: 'Release/CD 2/01.mp3' }),
            mkTrack({ path: 'Release/CD 1/02.mp3' }),
            mkTrack({ path: 'Release/CD 2/02.mp3' })
        ])
        expect(dirs).toEqual(['Release/CD 1', 'Release/CD 2'])
    })
})
