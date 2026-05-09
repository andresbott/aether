import { describe, it, expect } from 'vitest'
import {
    diffInitialValues,
    buildPatchFields
} from '@/composables/useMetadataEditor'
import type { Track } from '@/types/metadata'

const mkTrack = (over: Partial<Track> = {}): Track => ({
    path: 'a.mp3',
    name: 'a.mp3',
    title: '',
    artists: [],
    album_artists: [],
    album: '',
    year: 0,
    compilation: false,
    ...over
})

describe('diffInitialValues', () => {
    it('prefills when all selected tracks share the same value', () => {
        const a = mkTrack({ title: 'Hello', album: 'X', artists: ['One'] })
        const b = mkTrack({ path: 'b.mp3', title: 'Hello', album: 'X', artists: ['One'] })
        const v = diffInitialValues([a, b])
        expect(v.title).toEqual({ shared: true, value: 'Hello' })
        expect(v.album).toEqual({ shared: true, value: 'X' })
        expect(v.artists).toEqual({ shared: true, value: ['One'] })
    })

    it('marks fields as mixed when values differ', () => {
        const a = mkTrack({ title: 'Hello', album: 'X' })
        const b = mkTrack({ path: 'b.mp3', title: 'World', album: 'X' })
        const v = diffInitialValues([a, b])
        expect(v.title.shared).toBe(false)
        expect(v.album).toEqual({ shared: true, value: 'X' })
    })

    it('compares artist arrays by value, not reference', () => {
        const a = mkTrack({ artists: ['A', 'B'] })
        const b = mkTrack({ path: 'b.mp3', artists: ['A', 'B'] })
        const c = mkTrack({ path: 'c.mp3', artists: ['A', 'C'] })
        expect(diffInitialValues([a, b]).artists.shared).toBe(true)
        expect(diffInitialValues([a, c]).artists.shared).toBe(false)
    })
})

describe('buildPatchFields', () => {
    it('includes only enabled fields', () => {
        const applied = {
            title: true, album: false, artists: false, album_artists: false,
            year: true, compilation: false
        }
        const values = {
            title: 'Hi', album: 'unused', artists: ['unused'], album_artists: ['unused'],
            year: 2001, compilation: true
        }
        expect(buildPatchFields(applied, values)).toEqual({ title: 'Hi', year: 2001 })
    })

    it('returns an empty object when nothing is applied', () => {
        const applied = {
            title: false, album: false, artists: false, album_artists: false,
            year: false, compilation: false
        }
        const values = {
            title: 'a', album: 'b', artists: ['c'], album_artists: ['d'],
            year: 1, compilation: true
        }
        expect(buildPatchFields(applied, values)).toEqual({})
    })
})
