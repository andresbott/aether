/**
 * Pure math for the Now Playing sheet's drag gestures (NowPlayingSheet.vue).
 *
 * One position axis for the whole sheet: 0 = collapsed (mini strip), 1 =
 * playing (player face), 2 = queue. The two segments cover different finger
 * distances — 0→1 spans the viewport minus the strip already on screen, 1→2
 * spans a full viewport — so gestures map through cumulative TRAVEL px and
 * back, keeping the sheet 1:1 under the finger in both segments.
 *
 * No DOM here: NowPlayingSheet measures heights and feeds them in, which is
 * what makes flicks and clamping unit-testable.
 */

/** Movement below this is a tap (or the slider), never a drag claim. */
export const SLOP_PX = 8
/** Finger speed (px/ms) at which direction beats the midpoint rule. */
export const FLICK_VELOCITY_PX_MS = 0.5
/** Velocity looks at most this far back, so a pause before release kills a flick. */
const VELOCITY_WINDOW_MS = 100

const segments = (viewportH: number, stripH: number): [number, number] => [
    // max(1, …): jsdom reports 0 heights; gestures are inert there but the
    // math must not divide by zero.
    Math.max(1, viewportH - stripH),
    Math.max(1, viewportH)
]

/** Cumulative finger travel (px) from collapsed to `position`. */
export function travelFor(position: number, viewportH: number, stripH: number): number {
    const [expand, queue] = segments(viewportH, stripH)
    const p = Math.min(Math.max(position, 0), 2)
    return p <= 1 ? p * expand : expand + (p - 1) * queue
}

/** Inverse of `travelFor`, clamped to [0, 2]. */
export function positionAtTravel(travel: number, viewportH: number, stripH: number): number {
    const [expand, queue] = segments(viewportH, stripH)
    if (travel <= 0) return 0
    if (travel <= expand) return travel / expand
    return Math.min(2, 1 + (travel - expand) / queue)
}

/**
 * Position after the finger moved `deltaY` px (screen coordinates: down is
 * positive, which lowers the sheet), clamped to the surface's [min, max].
 */
export function dragPosition(
    startPos: number,
    deltaY: number,
    viewportH: number,
    stripH: number,
    min: number,
    max: number
): number {
    const travel = travelFor(startPos, viewportH, stripH) - deltaY
    const position = positionAtTravel(travel, viewportH, stripH)
    return Math.min(Math.max(position, min), max)
}

/**
 * Detent index to settle on at release: a flick moves one step in its
 * direction (from an exact detent too — the epsilon keeps floor/ceil from
 * treating "at 1" as "past 1"), anything slower rounds to the nearest.
 */
export function settleDetent(
    position: number,
    velocityY: number,
    min: number,
    max: number
): number {
    let target: number
    if (velocityY <= -FLICK_VELOCITY_PX_MS) target = Math.ceil(position + 1e-6)
    else if (velocityY >= FLICK_VELOCITY_PX_MS) target = Math.floor(position - 1e-6)
    else target = Math.round(position)
    return Math.min(Math.max(target, min), max)
}

/** Finger velocity over a trailing window, from touch event timestamps. */
export class VelocityTracker {
    private samples: Array<{ y: number; t: number }> = []

    push(y: number, t: number): void {
        this.samples.push({ y, t })
        const cutoff = t - VELOCITY_WINDOW_MS
        while (this.samples.length > 2 && this.samples[0].t < cutoff) {
            this.samples.shift()
        }
    }

    velocity(): number {
        if (this.samples.length < 2) return 0
        const first = this.samples[0]
        const last = this.samples[this.samples.length - 1]
        const dt = last.t - first.t
        return dt > 0 ? (last.y - first.y) / dt : 0
    }
}
