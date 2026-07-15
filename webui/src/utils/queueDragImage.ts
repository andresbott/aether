/**
 * Build the drag image shown under the cursor when dragging a multi-selection of
 * queue rows: the grabbed song's card, with an offset square peeking out behind
 * it to suggest a stack, and a badge with the number of songs being moved.
 *
 * The element is fully inline-styled (it is appended to <body>, outside any
 * component's scoped styles) and positioned off-screen by the caller's
 * `setDragImage` use. Title and cover are read from the grabbed row's DOM.
 */
export function buildMultiDragImage(dragEl: HTMLElement, count: number): HTMLElement {
    const title = dragEl.querySelector('.row-title')?.textContent ?? ''
    const coverImg = dragEl.querySelector('.row-cover img') as HTMLImageElement | null
    const coverSrc = coverImg?.src ?? ''

    const wrap = document.createElement('div')
    wrap.className = 'queue-drag-image'
    Object.assign(wrap.style, {
        position: 'fixed',
        top: '-1000px',
        left: '-1000px',
        width: '240px',
        pointerEvents: 'none'
    })

    // Offset square behind the card → "stack of songs".
    const stack = document.createElement('div')
    stack.className = 'queue-drag-image__stack'
    Object.assign(stack.style, {
        position: 'absolute',
        top: '8px',
        left: '8px',
        right: '-8px',
        bottom: '-8px',
        background: 'var(--app-surface, #ffffff)',
        border: '1px solid var(--app-border, #d4d4d8)',
        borderRadius: '8px'
    })
    wrap.appendChild(stack)

    const card = document.createElement('div')
    card.className = 'queue-drag-image__card'
    Object.assign(card.style, {
        position: 'relative',
        display: 'flex',
        alignItems: 'center',
        gap: '0.5rem',
        padding: '0.4rem 0.5rem',
        background: 'var(--app-surface, #ffffff)',
        border: '1px solid var(--app-border, #d4d4d8)',
        borderRadius: '8px',
        boxShadow: '0 4px 12px rgba(0, 0, 0, 0.15)'
    })

    const cover = document.createElement('div')
    cover.className = 'queue-drag-image__cover'
    Object.assign(cover.style, {
        width: '32px',
        height: '32px',
        flexShrink: '0',
        borderRadius: '4px',
        backgroundSize: 'cover',
        backgroundPosition: 'center',
        backgroundRepeat: 'no-repeat'
    })
    cover.style.backgroundImage = coverSrc
        ? `url("${coverSrc}")`
        : 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)'
    card.appendChild(cover)

    const label = document.createElement('span')
    label.className = 'queue-drag-image__title'
    label.textContent = title
    Object.assign(label.style, {
        fontSize: '0.85rem',
        fontWeight: '500',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis'
    })
    card.appendChild(label)

    wrap.appendChild(card)

    const badge = document.createElement('div')
    badge.className = 'queue-drag-image__badge'
    badge.textContent = String(count)
    Object.assign(badge.style, {
        position: 'absolute',
        top: '-8px',
        right: '-8px',
        minWidth: '20px',
        height: '20px',
        padding: '0 4px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'var(--app-accent, #0e9bb5)',
        color: '#ffffff',
        borderRadius: '10px',
        fontSize: '0.7rem',
        fontWeight: '700'
    })
    wrap.appendChild(badge)

    return wrap
}
