/**
 * Pure geometry for a responsive, virtualized card grid.
 *
 * The grid mirrors CSS `grid-template-columns: repeat(auto-fill, minmax(minColWidth, 1fr))`
 * with a fixed `gap`. Because the VirtualScroller renders one ROW per virtual item, callers
 * must translate the scroller's row ranges back into flat item ranges for lazy paging —
 * that is `rowRangeToItemRange`, which deliberately over-covers so no visible card is left
 * unloaded (page-level dedup absorbs the slack).
 */

export function computeGridColumns(
    availableWidth: number,
    minColWidth: number,
    gap: number
): number {
    if (availableWidth <= 0) return 1
    return Math.max(1, Math.floor((availableWidth + gap) / (minColWidth + gap)))
}

export function computeColumnWidth(availableWidth: number, columns: number, gap: number): number {
    if (columns <= 0) return 0
    return (availableWidth - (columns - 1) * gap) / columns
}

export function chunkRows<T>(items: (T | undefined)[], columns: number): (T | undefined)[][] {
    if (columns <= 0) return []
    const rows: (T | undefined)[][] = []
    for (let i = 0; i < items.length; i += columns) {
        rows.push(items.slice(i, i + columns))
    }
    return rows
}

export function offsetToRow(offset: number, columns: number): number {
    if (columns <= 0) return 0
    return Math.floor(Math.max(0, offset) / columns)
}

export function rowRangeToItemRange(
    firstRow: number,
    lastRow: number,
    columns: number,
    total: number
): { first: number; last: number } {
    if (columns <= 0 || total <= 0) return { first: 0, last: 0 }
    const first = Math.max(0, firstRow * columns)
    const last = Math.min(total - 1, (lastRow + 1) * columns - 1)
    return { first, last: Math.max(first, last) }
}
