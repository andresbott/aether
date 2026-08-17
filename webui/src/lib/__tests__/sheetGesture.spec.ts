import { describe, it, expect } from 'vitest'
import {
    dragPosition,
    positionAtTravel,
    settleDetent,
    travelFor,
    VelocityTracker,
    FLICK_VELOCITY_PX_MS,
    SLOP_PX
} from '@/lib/sheetGesture'

// One position axis for the whole sheet: 0 collapsed, 1 playing, 2 queue.
// Segment 0→1 spans (viewportH - stripH) finger-px (the strip is already on
// screen), segment 1→2 spans a full viewportH. deltaY is screen-down-positive.
const H = 800
const STRIP = 60

describe('travelFor / positionAtTravel', () => {
    it('maps the detents to their cumulative finger travel', () => {
        expect(travelFor(0, H, STRIP)).toBe(0)
        expect(travelFor(1, H, STRIP)).toBe(740)
        expect(travelFor(2, H, STRIP)).toBe(1540)
    })

    it('is piecewise linear inside each segment', () => {
        expect(travelFor(0.5, H, STRIP)).toBe(370)
        expect(travelFor(1.5, H, STRIP)).toBe(1140)
    })

    it('round-trips through its inverse', () => {
        for (const p of [0, 0.25, 0.5, 1, 1.3, 2]) {
            expect(positionAtTravel(travelFor(p, H, STRIP), H, STRIP)).toBeCloseTo(p, 10)
        }
    })

    it('clamps the inverse to [0, 2]', () => {
        expect(positionAtTravel(-50, H, STRIP)).toBe(0)
        expect(positionAtTravel(99999, H, STRIP)).toBe(2)
    })

    it('never divides by zero when jsdom reports no dimensions', () => {
        expect(positionAtTravel(travelFor(1, 0, 0), 0, 0)).toBeCloseTo(1, 10)
    })
})

describe('dragPosition', () => {
    it('moves up (finger dy negative) toward higher positions, 1:1 in travel', () => {
        // From collapsed, lifting 370px is half the 740px expand travel.
        expect(dragPosition(0, -370, H, STRIP, 0, 1)).toBeCloseTo(0.5, 10)
    })

    it('moves down toward lower positions', () => {
        expect(dragPosition(1, 370, H, STRIP, 0, 2)).toBeCloseTo(0.5, 10)
    })

    it('crosses segments in one gesture when the range allows it', () => {
        // From playing, lifting a full viewport reaches the queue.
        expect(dragPosition(1, -H, H, STRIP, 0, 2)).toBe(2)
    })

    it('clamps to the surface range', () => {
        // The strip only travels [0, 1]: overshoot stops at the face.
        expect(dragPosition(0, -2000, H, STRIP, 0, 1)).toBe(1)
        // The queue surfaces only travel [1, 2]: a hard pull stops at the face.
        expect(dragPosition(2, 2000, H, STRIP, 1, 2)).toBe(1)
    })
})

describe('settleDetent', () => {
    it('rounds to the nearest detent when the release is slow', () => {
        expect(settleDetent(0.4, 0, 0, 1)).toBe(0)
        expect(settleDetent(0.6, 0, 0, 1)).toBe(1)
        expect(settleDetent(1.5001, 0, 1, 2)).toBe(2)
    })

    it('a flick wins over the midpoint: direction decides', () => {
        // Barely moved, but flicked up hard → open anyway.
        expect(settleDetent(0.1, -FLICK_VELOCITY_PX_MS, 0, 1)).toBe(1)
        // Nearly open, but flicked down → dismiss.
        expect(settleDetent(0.9, FLICK_VELOCITY_PX_MS, 0, 1)).toBe(0)
    })

    it('a flick from an exact detent moves one step in its direction', () => {
        expect(settleDetent(1, -1, 0, 2)).toBe(2)
        expect(settleDetent(1, 1, 0, 2)).toBe(0)
    })

    it('clamps the target to the surface range', () => {
        expect(settleDetent(1, 1, 1, 2)).toBe(1)
        expect(settleDetent(1, -1, 0, 1)).toBe(1)
    })
})

describe('VelocityTracker', () => {
    it('reports px/ms over its sample window, sign preserved', () => {
        const t = new VelocityTracker()
        t.push(500, 0)
        t.push(480, 20)
        t.push(460, 40)
        expect(t.velocity()).toBeCloseTo(-1, 10)
    })

    it('drops samples older than its window so old motion cannot fake a flick', () => {
        const t = new VelocityTracker()
        t.push(500, 0) // fast start…
        t.push(400, 10)
        // …then the finger holds still for 300ms before release.
        t.push(400, 310)
        t.push(400, 320)
        expect(Math.abs(t.velocity())).toBeLessThan(FLICK_VELOCITY_PX_MS)
    })

    it('is zero with fewer than two samples or zero elapsed time', () => {
        const t = new VelocityTracker()
        expect(t.velocity()).toBe(0)
        t.push(100, 5)
        expect(t.velocity()).toBe(0)
        t.push(200, 5)
        expect(t.velocity()).toBe(0)
    })
})

describe('constants', () => {
    it('pins the tuning values the sheet component relies on', () => {
        expect(SLOP_PX).toBe(8)
        expect(FLICK_VELOCITY_PX_MS).toBe(0.5)
    })
})
