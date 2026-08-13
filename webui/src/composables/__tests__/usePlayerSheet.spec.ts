import { describe, it, expect, beforeEach, vi } from 'vitest'
import { usePlayerSheet, resetPlayerSheetForTests } from '../usePlayerSheet'

describe('usePlayerSheet', () => {
    let pushSpy: ReturnType<typeof vi.spyOn>
    let replaceSpy: ReturnType<typeof vi.spyOn>
    let backSpy: ReturnType<typeof vi.spyOn>
    // Stand-in for the browser's history stack: pushState/back only need to keep
    // `state` consistent for these specs, since that is what carries our marker.
    let stack: Array<unknown>

    beforeEach(() => {
        resetPlayerSheetForTests()
        stack = [{ base: true }]
        vi.spyOn(window.history, 'state', 'get').mockImplementation(
            () => stack[stack.length - 1]
        )
        pushSpy = vi.spyOn(window.history, 'pushState').mockImplementation((state) => {
            stack.push(state)
        })
        replaceSpy = vi.spyOn(window.history, 'replaceState').mockImplementation((state) => {
            stack[stack.length - 1] = state
        })
        backSpy = vi.spyOn(window.history, 'back').mockImplementation(() => {
            // A real back() fires popstate asynchronously; the composable must
            // not depend on that ordering, but the entry-consumption test does.
            stack.pop()
            window.dispatchEvent(
                new PopStateEvent('popstate', { state: stack[stack.length - 1] })
            )
        })
    })

    const popTo = (state: unknown): void => {
        window.dispatchEvent(new PopStateEvent('popstate', { state }))
    }

    it('starts closed', () => {
        expect(usePlayerSheet().isOpen.value).toBe(false)
    })

    it('open() opens and pushes exactly one history entry', () => {
        const sheet = usePlayerSheet()
        sheet.open()
        sheet.open() // repeat is a no-op
        expect(sheet.isOpen.value).toBe(true)
        expect(pushSpy).toHaveBeenCalledTimes(1)
    })

    it('open() preserves the existing history state alongside its marker', () => {
        stack = [{ scroll: { top: 42 }, position: 3 }]
        usePlayerSheet().open()
        const pushed = pushSpy.mock.calls[0][0] as Record<string, unknown>
        expect(pushed.scroll).toEqual({ top: 42 })
        expect(pushed.position).toBe(3)
        expect(typeof pushed.aetherPlayerSheet).toBe('number')
    })

    it('system back closes the sheet without further history calls', () => {
        const sheet = usePlayerSheet()
        sheet.open()
        stack.pop()
        popTo(stack[stack.length - 1])
        expect(sheet.isOpen.value).toBe(false)
        expect(backSpy).not.toHaveBeenCalled()
    })

    it('close() consumes the pushed entry via history.back()', () => {
        const sheet = usePlayerSheet()
        sheet.open()
        sheet.close()
        expect(sheet.isOpen.value).toBe(false)
        expect(backSpy).toHaveBeenCalledTimes(1)
    })

    it('double close() calls back() only once', () => {
        const sheet = usePlayerSheet()
        sheet.open()
        sheet.close()
        sheet.close()
        expect(backSpy).toHaveBeenCalledTimes(1)
    })

    it('still opens and closes when pushState throws', () => {
        pushSpy.mockImplementation(() => {
            throw new Error('denied')
        })
        const sheet = usePlayerSheet()
        sheet.open()
        expect(sheet.isOpen.value).toBe(true)
        sheet.close()
        expect(sheet.isOpen.value).toBe(false)
        expect(backSpy).not.toHaveBeenCalled() // nothing was pushed, nothing to consume
    })

    it('close(callback) invokes callback after popstate from back()', () => {
        const sheet = usePlayerSheet()
        const callback = vi.fn()
        sheet.open()
        sheet.close(callback)
        // backSpy synchronously dispatches popstate, so callback runs before this line
        expect(callback).toHaveBeenCalledTimes(1)
        expect(backSpy).toHaveBeenCalledTimes(1)
        // Callback must run AFTER back() so navigation happens after entry consumption
        const backCallOrder = backSpy.mock.invocationCallOrder[0]
        const callbackCallOrder = callback.mock.invocationCallOrder[0]
        expect(callbackCallOrder).toBeGreaterThan(backCallOrder)
    })

    it('close(callback) calls callback synchronously when nothing was pushed', () => {
        pushSpy.mockImplementation(() => {
            throw new Error('denied')
        })
        const sheet = usePlayerSheet()
        const callback = vi.fn()
        sheet.open()
        sheet.close(callback)
        expect(callback).toHaveBeenCalledTimes(1)
        expect(backSpy).not.toHaveBeenCalled()
    })

    it('close(callback) runs the callback exactly once — no timer double-fire', async () => {
        vi.useFakeTimers()
        try {
            const sheet = usePlayerSheet()
            const callback = vi.fn()
            sheet.open()
            sheet.close(callback)
            expect(callback).toHaveBeenCalledTimes(1)
            // Any wall-clock fallback would fire a second navigation here.
            await vi.advanceTimersByTimeAsync(2000)
            expect(callback).toHaveBeenCalledTimes(1)
        } finally {
            vi.useRealTimers()
        }
    })

    it('close(callback) still runs the callback when popstate is async', async () => {
        // Real browsers deliver popstate on a later task; the callback must wait
        // for it rather than for a timeout.
        backSpy.mockImplementation(() => {
            stack.pop()
            setTimeout(() => popTo(stack[stack.length - 1]), 0)
        })
        const sheet = usePlayerSheet()
        const callback = vi.fn()
        sheet.open()
        sheet.close(callback)
        expect(callback).not.toHaveBeenCalled()
        await new Promise((resolve) => setTimeout(resolve, 5))
        expect(callback).toHaveBeenCalledTimes(1)
    })

    describe('dismiss() — non-consuming dismissal', () => {
        it('closes without touching history.back()', () => {
            const sheet = usePlayerSheet()
            sheet.open()
            sheet.dismiss()
            expect(sheet.isOpen.value).toBe(false)
            expect(backSpy).not.toHaveBeenCalled()
        })

        it('strips the marker from the current entry so it cannot look live', () => {
            const sheet = usePlayerSheet()
            sheet.open()
            sheet.dismiss()
            expect(replaceSpy).toHaveBeenCalledTimes(1)
            const replaced = replaceSpy.mock.calls[0][0] as Record<string, unknown>
            expect(replaced.aetherPlayerSheet).toBeUndefined()
            expect(replaced.base).toBe(true) // rest of the state survives
        })

        it('leaves a later system back free to navigate (nothing swallowed)', () => {
            const sheet = usePlayerSheet()
            sheet.open()
            sheet.dismiss()
            // The user navigates on, then goes back: our listener must be gone,
            // and even if it fires it must not call back() itself.
            stack.pop()
            popTo(stack[stack.length - 1])
            expect(backSpy).not.toHaveBeenCalled()
            expect(sheet.isOpen.value).toBe(false)
        })

        it('is a no-op while closed', () => {
            const sheet = usePlayerSheet()
            sheet.dismiss()
            expect(replaceSpy).not.toHaveBeenCalled()
            expect(backSpy).not.toHaveBeenCalled()
        })
    })

    describe('close/open race', () => {
        it('an in-flight popstate from the close does not shut the reopened sheet', async () => {
            backSpy.mockImplementation(() => {
                stack.pop()
                setTimeout(() => popTo(stack[stack.length - 1]), 0)
            })
            const sheet = usePlayerSheet()
            sheet.open()
            sheet.close()
            sheet.open() // reopened before the first popstate lands
            expect(sheet.isOpen.value).toBe(true)
            await new Promise((resolve) => setTimeout(resolve, 5))
            expect(sheet.isOpen.value).toBe(true)
            expect(pushSpy).toHaveBeenCalledTimes(2)
        })

        it('the reopened sheet still closes on the next real back', async () => {
            backSpy.mockImplementation(() => {
                stack.pop()
                setTimeout(() => popTo(stack[stack.length - 1]), 0)
            })
            const sheet = usePlayerSheet()
            sheet.open()
            sheet.close()
            sheet.open()
            await new Promise((resolve) => setTimeout(resolve, 5))
            stack.pop()
            popTo(stack[stack.length - 1])
            expect(sheet.isOpen.value).toBe(false)
        })
    })
})
