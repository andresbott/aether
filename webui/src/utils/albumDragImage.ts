/**
 * Build the drag image shown under the cursor when dragging an album into the
 * queue: the album cover thumbnail, name + artist, and a song-count badge.
 * Distinct from the multi-song reorder "stack" image. Fully inline-styled (it
 * is appended to <body>, outside any component's scoped styles) and positioned
 * off-screen by the caller's `setDragImage` use.
 */
export interface AlbumDragImageData {
    coverSrc: string | null
    name: string
    artist: string
    count: number
}

export function buildAlbumDragImage(data: AlbumDragImageData): HTMLElement {
    const wrap = document.createElement('div')
    wrap.className = 'album-drag-image'
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
    cover.className = 'album-drag-image__cover'
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
    title.className = 'album-drag-image__title'
    title.textContent = data.name
    Object.assign(title.style, {
        fontSize: '0.9rem',
        fontWeight: '600',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis'
    })
    info.appendChild(title)

    const artist = document.createElement('div')
    artist.className = 'album-drag-image__artist'
    artist.textContent = data.artist
    Object.assign(artist.style, {
        fontSize: '0.78rem',
        color: 'var(--app-text-secondary, #71717a)',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis'
    })
    info.appendChild(artist)
    wrap.appendChild(info)

    const badge = document.createElement('div')
    badge.className = 'album-drag-image__badge'
    badge.textContent = String(data.count)
    Object.assign(badge.style, {
        flexShrink: '0',
        minWidth: '22px',
        height: '22px',
        padding: '0 6px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'var(--app-accent, #6366f1)',
        color: '#ffffff',
        borderRadius: '11px',
        fontSize: '0.72rem',
        fontWeight: '700'
    })
    wrap.appendChild(badge)

    return wrap
}
