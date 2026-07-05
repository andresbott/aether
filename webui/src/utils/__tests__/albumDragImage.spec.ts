import { describe, it, expect } from 'vitest'
import { buildAlbumDragImage } from '@/utils/albumDragImage'

describe('buildAlbumDragImage', () => {
    it('renders the album name, artist and song-count badge', () => {
        const el = buildAlbumDragImage({
            coverSrc: null,
            name: 'Dark Side of the Moon',
            artist: 'Pink Floyd',
            count: 12
        })
        expect(el.querySelector('.album-drag-image__title')?.textContent).toBe(
            'Dark Side of the Moon'
        )
        expect(el.querySelector('.album-drag-image__artist')?.textContent).toBe('Pink Floyd')
        expect(el.querySelector('.album-drag-image__badge')?.textContent).toBe('12')
    })

    it('uses the cover art as the thumbnail background when present', () => {
        const el = buildAlbumDragImage({
            coverSrc: 'http://x/cover.jpg',
            name: 'LP',
            artist: 'A',
            count: 1
        })
        const cover = el.querySelector('.album-drag-image__cover') as HTMLElement
        expect(cover.style.backgroundImage).toContain('cover.jpg')
    })

    it('falls back to a gradient when there is no cover', () => {
        const el = buildAlbumDragImage({ coverSrc: null, name: 'LP', artist: 'A', count: 1 })
        const cover = el.querySelector('.album-drag-image__cover') as HTMLElement
        expect(cover.style.backgroundImage).toContain('gradient')
    })
})
