// @vitest-environment node
// Scoped styles never apply under vue-test-utils, so the phone column collapse is
// pinned off disk (same technique as ContentScaffold.compactStyles.spec.ts).
//
// The rows hide cells; the HOSTS own the grid template (--album-track-cols /
// --genre-track-cols) and the header cells. Cell count and template have to agree
// at BOTH widths, so this spec pins the row half and the host half together —
// hiding a cell without shrinking a host's template misaligns every row.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const read = (rel: string): string =>
    readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')

const phoneMedia = (source: string): string | undefined =>
    source.match(/@media \(max-width: 767\.98px\)\s*\{[\s\S]*?\n\}/)?.[0]

const coarseMedia = (source: string): string | undefined =>
    source.match(/@media \(pointer: coarse\)\s*\{[\s\S]*?\n\}/)?.[0]

describe('track rows collapse low-value columns on phones', () => {
    it('AlbumTrackRow hides the artist cell', () => {
        const media = phoneMedia(read('../AlbumTrackRow.vue'))
        expect(media).toBeTruthy()
        expect(media).toMatch(/\.col-artist[\s\S]*?\{[^}]*display:\s*none/)
    })

    it('GenreTrackRow hides the album and artist cells but keeps the cover', () => {
        const source = read('../GenreTrackRow.vue')
        const media = phoneMedia(source)
        expect(media).toBeTruthy()
        expect(media).toMatch(/\.col-artist,\s*\n?\s*\.col-album\s*\{[^}]*display:\s*none/)
        expect(media).not.toMatch(/\.col-cover\s*\{[^}]*display:\s*none/)
    })
})

// `TrackFavoriteButton` ships `.row-star { opacity: 0 }` and each host row owns the
// reveal. A row that carries only the `:hover` / `:focus-visible` half has an
// INVISIBLE heart on touch, where neither ever fires — which is exactly what the
// queue shipped with. All three track rows have to carry the coarse rule.
// See docs/architecture/unified-play-experience.md.
describe('track rows reveal the favorite heart on touch', () => {
    const rows: { name: string; file: string; cell: RegExp }[] = [
        { name: 'AlbumTrackRow', file: '../AlbumTrackRow.vue', cell: /\.col-star/ },
        { name: 'GenreTrackRow', file: '../GenreTrackRow.vue', cell: /\.col-star/ },
        { name: 'QueueRow', file: '../../layout/QueueRow.vue', cell: /\.row-star-cell/ }
    ]

    for (const { name, file, cell } of rows) {
        it(`${name} pins .row-star visible under (pointer: coarse)`, () => {
            const media = coarseMedia(read(file))
            expect(media).toBeTruthy()
            expect(media).toMatch(cell)
            expect(media).toMatch(/\.row-star\)?\s*\{[^}]*opacity:\s*1/)
        })
    }
})

describe('hosts shrink their grid template to match', () => {
    // The album template loses one track (artist); the genre template loses two
    // (artist + album). Both keep index/cover, title, select, star, duration.
    const cases: { file: string; varName: string; cols: RegExp }[] = [
        {
            file: '../../../views/AlbumView.vue',
            varName: '--album-track-cols',
            cols: /--album-track-cols:\s*38px minmax\(0, 1fr\) 2rem 2rem 62px/
        },
        {
            file: '../../../views/GenreDetailView.vue',
            varName: '--genre-track-cols',
            cols: /--genre-track-cols:\s*48px minmax\(0, 1fr\) 2rem 2rem 62px/
        },
        {
            file: '../../../views/PlaylistDetailView.vue',
            varName: '--genre-track-cols',
            cols: /--genre-track-cols:\s*48px minmax\(0, 1fr\) 2rem 2rem 62px/
        },
        {
            file: '../../../views/SearchView.vue',
            varName: '--genre-track-cols',
            cols: /--genre-track-cols:\s*48px minmax\(0, 1fr\) 2rem 2rem 62px/
        }
    ]

    for (const { file, varName, cols } of cases) {
        const name = file.split('/').pop()

        it(`${name} redefines ${varName} on phones and hides the matching headers`, () => {
            const media = phoneMedia(read(file))
            expect(media).toBeTruthy()
            expect(media).toMatch(cols)
            expect(media).toMatch(/\.track-list-header .col-artist[\s\S]*?display:\s*none/)
            if (varName === '--genre-track-cols') {
                expect(media).toMatch(/\.track-list-header .col-album[\s\S]*?display:\s*none/)
            }
        })
    }
})
