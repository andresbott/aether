/**
 * The queue index to insert a dropped block before. When there is a following
 * row, use its index. With no following row, the drop is at the end of a list:
 * the history list ends just before the current track (→ currentIndex); the
 * upcoming list ends at the tail of the queue (→ queueLength).
 */
export function computeDropTarget(
    anchorIndex: number | undefined,
    isHistory: boolean,
    currentIndex: number,
    queueLength: number
): number {
    if (anchorIndex !== undefined) return anchorIndex
    return isHistory ? currentIndex : queueLength
}

/**
 * Reorder `queue` by removing the items at `fromIndices` and reinserting them
 * as one contiguous block immediately before the first NON-moved item whose
 * original index is `>= targetIndex`. If there is no such item (target points
 * past the end, or only moved items remain after it), the block is appended.
 *
 * Pure: returns a new array and preserves element references (so callers can
 * re-find an element by identity after the move).
 */
export function reorderQueue<T>(queue: T[], fromIndices: number[], targetIndex: number): T[] {
    const movedSet = new Set(fromIndices)
    if (movedSet.size === 0) return [...queue]

    const moved = [...movedSet].sort((a, b) => a - b).map((i) => queue[i])

    const rest: T[] = []
    let insertAt = -1
    for (let i = 0; i < queue.length; i++) {
        if (movedSet.has(i)) continue
        if (insertAt === -1 && i >= targetIndex) insertAt = rest.length
        rest.push(queue[i])
    }
    if (insertAt === -1) insertAt = rest.length

    return [...rest.slice(0, insertAt), ...moved, ...rest.slice(insertAt)]
}
