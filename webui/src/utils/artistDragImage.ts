/**
 * Build the drag image shown under the cursor when dragging an artist into the
 * queue: the artist image thumbnail, name, and an album-count badge. Mirrors the
 * album drag image pattern. Fully inline-styled (it is appended to <body>, outside
 * any component's scoped styles) and positioned off-screen by the caller's
 * `setDragImage` use.
 */
export interface ArtistDragImageData {
    coverSrc: string | null
    name: string
    albumCount: number
}

export function buildArtistDragImage(data: ArtistDragImageData): HTMLElement {
    const wrap = document.createElement('div')
    wrap.className = 'artist-drag-image'
    Object.assign(wrap.style, {
        position: 'fixed',
        top: '-1000px',
        left: '-1000px',
        width: '260px',
        display: 'flex',
        alignItems: 'center',
        gap: '0.6rem',
        padding: '0.5rem 0.6rem',
        background: 'var(--app-surface, #ffffff)',
        border: '1px solid var(--app-border, #d4d4d8)',
        borderRadius: '8px',
        boxShadow: '0 4px 12px rgba(0, 0, 0, 0.15)',
        pointerEvents: 'none'
    })

    const cover = document.createElement('div')
    cover.className = 'artist-drag-image__cover'
    Object.assign(cover.style, {
        width: '44px',
        height: '44px',
        flexShrink: '0',
        borderRadius: '4px',
        backgroundSize: 'cover',
        backgroundPosition: 'center',
        backgroundRepeat: 'no-repeat'
    })
    cover.style.backgroundImage = data.coverSrc
        ? `url("${data.coverSrc}")`
        : 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)'
    wrap.appendChild(cover)

    const info = document.createElement('div')
    Object.assign(info.style, { minWidth: '0', flex: '1' })

    const title = document.createElement('div')
    title.className = 'artist-drag-image__title'
    title.textContent = data.name
    Object.assign(title.style, {
        fontSize: '0.9rem',
        fontWeight: '600',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis'
    })
    info.appendChild(title)

    const subtitle = document.createElement('div')
    subtitle.className = 'artist-drag-image__subtitle'
    subtitle.textContent = `${data.albumCount} ${data.albumCount === 1 ? 'album' : 'albums'}`
    Object.assign(subtitle.style, {
        fontSize: '0.78rem',
        color: 'var(--app-text-secondary, #71717a)',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis'
    })
    info.appendChild(subtitle)
    wrap.appendChild(info)

    const badge = document.createElement('div')
    badge.className = 'artist-drag-image__badge'
    badge.textContent = String(data.albumCount)
    Object.assign(badge.style, {
        flexShrink: '0',
        minWidth: '22px',
        height: '22px',
        padding: '0 6px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'var(--app-accent, #0e9bb5)',
        color: '#ffffff',
        borderRadius: '11px',
        fontSize: '0.72rem',
        fontWeight: '700'
    })
    wrap.appendChild(badge)

    return wrap
}
