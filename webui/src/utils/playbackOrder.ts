/**
 * The queue index that should play after `currentIndex`, honoring the repeat
 * mode. Returns null when playback should stop — past the last track with
 * repeat 'none', or an empty queue. Repeat 'all' wraps back to the first track.
 */
export function nextQueueIndex(
    currentIndex: number,
    queueLength: number,
    repeat: 'none' | 'all'
): number | null {
    if (queueLength <= 0) return null
    const next = currentIndex + 1
    if (next >= queueLength) {
        return repeat === 'all' ? 0 : null
    }
    return next
}

/** A Fisher-Yates shuffled copy of `items`. Never mutates the input. */
export function shuffleItems<T>(items: readonly T[], rand: () => number = Math.random): T[] {
    const out = [...items]
    for (let i = out.length - 1; i > 0; i--) {
        const j = Math.floor(rand() * (i + 1))
        const tmp = out[i] as T
        out[i] = out[j] as T
        out[j] = tmp
    }
    return out
}

/**
 * A random play order over `items` — a full permutation kept alongside the
 * queue, so shuffled next/previous walk a stable sequence instead of drawing a
 * fresh random track each time. `first` is placed at the head when present, so
 * the order starts at whatever is playing and nothing before it is skipped.
 */
export function buildShuffleOrder<T>(
    items: readonly T[],
    first: T | null,
    rand: () => number = Math.random
): T[] {
    if (first === null || !items.includes(first)) return shuffleItems(items, rand)
    return [
        first,
        ...shuffleItems(
            items.filter((i) => i !== first),
            rand
        )
    ]
}

/**
 * `order` brought back in line with `items` after the queue changed: entries no
 * longer queued are dropped and newly queued ones are spliced in at random
 * positions after `current`, so additions land somewhere in the part of the
 * random order that has not played yet. The relative order of surviving entries
 * is preserved, which keeps the upcoming sequence stable across queue edits.
 */
export function resyncShuffleOrder<T>(
    order: readonly T[],
    items: readonly T[],
    current: T | null,
    rand: () => number = Math.random
): T[] {
    const queued = new Set(items)
    const kept = order.filter((o) => queued.has(o))
    const known = new Set(kept)
    const added = shuffleItems(
        items.filter((i) => !known.has(i)),
        rand
    )
    if (added.length === 0) return kept

    const cursor = current === null ? -1 : kept.indexOf(current)
    const head = kept.slice(0, cursor + 1)
    const tail = kept.slice(cursor + 1)
    for (const item of added) {
        tail.splice(Math.floor(rand() * (tail.length + 1)), 0, item)
    }
    return [...head, ...tail]
}

/**
 * The position to move to from `position` within a play order of `length`
 * entries. Returns null when the step falls off either end and repeat is
 * 'none'; repeat 'all' wraps around. A `position` of -1 (the playing track is
 * not in the order) steps to the first or last entry.
 */
export function stepOrderPosition(
    position: number,
    length: number,
    delta: 1 | -1,
    repeat: 'none' | 'all'
): number | null {
    if (length <= 0) return null
    if (position < 0) return delta === 1 ? 0 : length - 1
    const next = position + delta
    if (next >= 0 && next < length) return next
    if (repeat !== 'all') return null
    return (next + length) % length
}
