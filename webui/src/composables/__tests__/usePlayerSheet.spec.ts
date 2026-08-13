import { describe, it, expect, beforeEach, vi } from 'vitest'
import { usePlayerSheet, resetPlayerSheetForTests } from '../usePlayerSheet'

describe('usePlayerSheet', () => {
    let pushSpy: ReturnType<typeof vi.spyOn>
    let backSpy: ReturnType<typeof vi.spyOn>

    beforeEach(() => {
        resetPlayerSheetForTests()
        pushSpy = vi.spyOn(window.history, 'pushState').mockImplementation(() => {})
        backSpy = vi.spyOn(window.history, 'back').mockImplementation(() => {
            // A real back() fires popstate asynchronously; the composable must
            // not depend on that ordering, but the entry-consumption test does.
            window.dispatchEvent(new PopStateEvent('popstate'))
        })
    })

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

    it('system back closes the sheet without further history calls', () => {
        const sheet = usePlayerSheet()
        sheet.open()
        window.dispatchEvent(new PopStateEvent('popstate'))
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
})
