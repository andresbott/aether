export interface QueueRowRect {
    queueIndex: number
    top: number
    bottom: number
}

/**
 * Map a pointer Y position to the queue array index at which a dropped block
 * should be inserted: before the first row whose vertical midpoint is below the
 * pointer, otherwise at the end (`queueLength`). `rows` must be sorted by
 * `queueIndex` ascending. Pure.
 */
export function computeInsertIndex(
    rows: QueueRowRect[],
    pointerY: number,
    queueLength: number
): number {
    for (const row of rows) {
        const mid = (row.top + row.bottom) / 2
        if (pointerY < mid) return row.queueIndex
    }
    return queueLength
}
