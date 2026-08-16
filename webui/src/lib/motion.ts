/**
 * Whether the user has asked for reduced motion.
 *
 * Feature-detected: jsdom has no `matchMedia`, and defaulting to `false`
 * (animate) matches what the CSS does without the query.
 *
 * Read at the moment a motion starts rather than tracked reactively — the
 * gestures that consult it (`MobilePlayView`'s drag down to `/browse`,
 * `MiniPlayer`'s lift to Now Playing) only need the answer when they commit, and
 * both have to skip their slide-out entirely rather than animate faster: a
 * transition that never runs never fires `transitionend`, so anything waiting on
 * that event has to be told up front.
 */
export function prefersReducedMotion(): boolean {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false
    return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}
