import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'

type UseConnectivity = (typeof import('@/composables/useConnectivity'))['useConnectivity']
let useConnectivity: UseConnectivity
let reportNetworkError: () => void
let dismissBanner: () => void

beforeEach(async () => {
    vi.resetModules()
    // Stub window for SSR-safe tests
    vi.stubGlobal('window', {
        addEventListener: vi.fn(),
        removeEventListener: vi.fn()
    })
    const mod = await import('@/composables/useConnectivity')
    useConnectivity = mod.useConnectivity
    reportNetworkError = mod.reportNetworkError
    dismissBanner = mod.dismissBanner
})

afterEach(() => {
    vi.unstubAllGlobals()
})

describe('useConnectivity', () => {
    it('starts with isOffline false', () => {
        const { isOffline } = useConnectivity()
        expect(isOffline.value).toBe(false)
    })

    it('reportNetworkError sets isOffline to true', () => {
        const { isOffline } = useConnectivity()
        expect(isOffline.value).toBe(false)
        reportNetworkError()
        expect(isOffline.value).toBe(true)
    })

    it('reportNetworkError is idempotent — multiple calls keep it true', () => {
        const { isOffline } = useConnectivity()
        reportNetworkError()
        expect(isOffline.value).toBe(true)
        reportNetworkError()
        expect(isOffline.value).toBe(true)
    })

    it('dismissBanner sets isOffline to false', () => {
        const { isOffline } = useConnectivity()
        reportNetworkError()
        expect(isOffline.value).toBe(true)
        dismissBanner()
        expect(isOffline.value).toBe(false)
    })

    it('dismissBanner after dismiss is harmless', () => {
        const { isOffline } = useConnectivity()
        expect(isOffline.value).toBe(false)
        dismissBanner()
        expect(isOffline.value).toBe(false)
    })
})

describe('useConnectivity window event handlers', () => {
    it('registers offline and online listeners once at module scope', async () => {
        const addEventListener = vi.fn()
        vi.stubGlobal('window', { addEventListener, removeEventListener: vi.fn() })

        // First import — should register listeners
        vi.resetModules()
        await import('@/composables/useConnectivity')
        const calls1 = addEventListener.mock.calls.length

        // Second import of the same module — should not register again (singleton)
        const mod = await import('@/composables/useConnectivity')
        mod.useConnectivity()
        const calls2 = addEventListener.mock.calls.length

        expect(calls2).toBe(calls1) // No duplicate listeners
        expect(calls1).toBeGreaterThanOrEqual(2) // At least offline + online
        expect(addEventListener).toHaveBeenCalledWith('offline', expect.any(Function))
        expect(addEventListener).toHaveBeenCalledWith('online', expect.any(Function))
    })

    it('window offline event triggers reportNetworkError', async () => {
        let offlineHandler: (() => void) | undefined
        const addEventListener = vi.fn((event: string, handler: () => void) => {
            if (event === 'offline') offlineHandler = handler
        })
        vi.stubGlobal('window', { addEventListener, removeEventListener: vi.fn() })

        vi.resetModules()
        const { useConnectivity } = await import('@/composables/useConnectivity')
        const { isOffline } = useConnectivity()

        expect(isOffline.value).toBe(false)
        offlineHandler?.()
        expect(isOffline.value).toBe(true)
    })

    it('window online event triggers dismissBanner', async () => {
        let onlineHandler: (() => void) | undefined
        const addEventListener = vi.fn((event: string, handler: () => void) => {
            if (event === 'online') onlineHandler = handler
        })
        vi.stubGlobal('window', { addEventListener, removeEventListener: vi.fn() })

        vi.resetModules()
        const { useConnectivity, reportNetworkError } = await import('@/composables/useConnectivity')
        const { isOffline } = useConnectivity()

        reportNetworkError()
        expect(isOffline.value).toBe(true)
        onlineHandler?.()
        expect(isOffline.value).toBe(false)
    })
})

describe('useConnectivity probe', () => {
    type Mod = typeof import('@/composables/useConnectivity')
    let mod: Mod

    beforeEach(async () => {
        vi.useFakeTimers()
        vi.resetModules()
        mod = await import('@/composables/useConnectivity')
    })

    afterEach(() => {
        mod.resetConnectivityForTests()
        vi.useRealTimers()
    })

    it('probes on a backoff while offline and recovers when the server answers', async () => {
        const probe = vi.fn().mockResolvedValueOnce(false).mockResolvedValue(true)
        const reconnected = vi.fn()
        mod.setConnectivityProbe(probe)
        mod.onReconnect(reconnected)
        const { isOffline } = mod.useConnectivity()

        mod.reportNetworkError()
        await vi.advanceTimersByTimeAsync(2_000)
        expect(probe).toHaveBeenCalledTimes(1)
        expect(isOffline.value).toBe(true)
        expect(reconnected).not.toHaveBeenCalled()

        await vi.advanceTimersByTimeAsync(5_000)
        expect(probe).toHaveBeenCalledTimes(2)
        expect(isOffline.value).toBe(false)
        expect(reconnected).toHaveBeenCalledTimes(1)

        // Recovered: no more probing.
        await vi.advanceTimersByTimeAsync(60_000)
        expect(probe).toHaveBeenCalledTimes(2)
    })

    // A phone's 'online' event (or coming back to the page) is a hint, not proof.
    it('checks at once on a hint instead of trusting it', async () => {
        const probe = vi.fn().mockResolvedValue(false)
        mod.setConnectivityProbe(probe)
        const { isOffline } = mod.useConnectivity()
        mod.reportNetworkError()

        mod.checkConnectivityNow()
        await vi.advanceTimersByTimeAsync(0)
        expect(probe).toHaveBeenCalledTimes(1)
        expect(isOffline.value).toBe(true)
    })

    it('does not probe while online', async () => {
        const probe = vi.fn().mockResolvedValue(true)
        mod.setConnectivityProbe(probe)
        mod.checkConnectivityNow()
        await vi.advanceTimersByTimeAsync(60_000)
        expect(probe).not.toHaveBeenCalled()
    })

    it('runs the reconnect listeners when a request succeeds during an outage', () => {
        const reconnected = vi.fn()
        mod.onReconnect(reconnected)
        mod.reportNetworkError()
        mod.markOnline()
        expect(reconnected).toHaveBeenCalledTimes(1)
        // Already online: nothing to recover from.
        mod.markOnline()
        expect(reconnected).toHaveBeenCalledTimes(1)
    })

    it('keeps probing when the probe throws', async () => {
        const probe = vi.fn().mockRejectedValueOnce(new Error('boom')).mockResolvedValue(true)
        mod.setConnectivityProbe(probe)
        const { isOffline } = mod.useConnectivity()
        mod.reportNetworkError()
        await vi.advanceTimersByTimeAsync(2_000 + 5_000)
        expect(probe).toHaveBeenCalledTimes(2)
        expect(isOffline.value).toBe(false)
    })
})
