import { describe, it, expect } from 'vitest'
import { buildMultiDragImage } from '@/utils/queueDragImage'

const fakeRow = (title: string, coverSrc?: string): HTMLElement => {
    const row = document.createElement('div')
    const t = document.createElement('span')
    t.className = 'row-title'
    t.textContent = title
    row.appendChild(t)
    if (coverSrc) {
        const cover = document.createElement('span')
        cover.className = 'row-cover'
        const img = document.createElement('img')
        img.src = coverSrc
        cover.appendChild(img)
        row.appendChild(cover)
    }
    return row
}

describe('buildMultiDragImage', () => {
    it('renders the grabbed song title and the selection count badge', () => {
        const img = buildMultiDragImage(fakeRow('Song A'), 3)
        expect(img.querySelector('.queue-drag-image__title')?.textContent).toBe('Song A')
        expect(img.querySelector('.queue-drag-image__badge')?.textContent).toBe('3')
    })

    it('includes an offset square behind the card to suggest a stack', () => {
        const img = buildMultiDragImage(fakeRow('Song A'), 2)
        expect(img.querySelector('.queue-drag-image__stack')).not.toBeNull()
        expect(img.querySelector('.queue-drag-image__card')).not.toBeNull()
    })

    it('uses the row cover art as the card background when present', () => {
        const img = buildMultiDragImage(fakeRow('Song A', 'http://x/cover.jpg'), 2)
        const cover = img.querySelector('.queue-drag-image__cover') as HTMLElement
        expect(cover.style.backgroundImage).toContain('cover.jpg')
    })
})
