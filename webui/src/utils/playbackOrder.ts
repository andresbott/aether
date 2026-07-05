/**
 * The queue index that should play after `currentIndex`, honoring the repeat
 * mode. Returns null when playback should stop — past the last track with
 * repeat 'none', or an empty queue. Repeat 'all' wraps back to the first track.
 *
 * Repeat 'one' is intentionally not handled here: replaying the current track
 * is the caller's concern, not a linear progression of the queue.
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
