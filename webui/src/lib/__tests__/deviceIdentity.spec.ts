import { describe, it, expect, beforeEach, afterEach } from 'vitest'

// The device identity is module-cached, so tests that need a "different
// browser" reload the module with storage already cleared.
async function freshModule() {
    const mod = await import('@/lib/deviceIdentity')
    return mod
}

function stubUserAgent(ua: string): void {
    Object.defineProperty(window.navigator, 'userAgent', { value: ua, configurable: true })
}

const realUserAgent = window.navigator.userAgent

describe('deviceIdentity', () => {
    beforeEach(async () => {
        const { vi } = await import('vitest')
        vi.resetModules()
        localStorage.clear()
    })

    afterEach(() => {
        stubUserAgent(realUserAgent)
    })

    describe('getDeviceId', () => {
        it('persists the generated id so the same browser keeps one session', async () => {
            const { getDeviceId, DEVICE_ID_KEY } = await freshModule()
            const id = getDeviceId()
            expect(id).not.toBe('')
            expect(JSON.parse(localStorage.getItem(DEVICE_ID_KEY) as string)).toBe(id)
        })

        it('returns the stored id after a reload instead of minting a new device', async () => {
            const first = await freshModule()
            const id = first.getDeviceId()

            const { vi } = await import('vitest')
            vi.resetModules()
            const second = await freshModule()
            expect(second.getDeviceId()).toBe(id)
        })

        it('gives a browser with empty storage its own id', async () => {
            const first = await freshModule()
            const id = first.getDeviceId()

            const { vi } = await import('vitest')
            vi.resetModules()
            localStorage.clear() // a different browser: nothing stored yet
            const second = await freshModule()
            expect(second.getDeviceId()).not.toBe(id)
        })

        // The id rides a scope string on the server, which rejects anything
        // outside [A-Za-z0-9_-] and longer than 64 characters.
        it('generates an id the server accepts as a scope segment', async () => {
            const { getDeviceId } = await freshModule()
            const id = getDeviceId()
            expect(id).toMatch(/^[A-Za-z0-9_-]{1,64}$/)
        })

        // A tampered or foreign value would 400 the mint and blank the player.
        it('replaces a stored id the server would reject', async () => {
            const { DEVICE_ID_KEY } = await freshModule()
            localStorage.setItem(DEVICE_ID_KEY, JSON.stringify('not a valid id!'))

            const { vi } = await import('vitest')
            vi.resetModules()
            const { getDeviceId } = await freshModule()
            const id = getDeviceId()
            expect(id).toMatch(/^[A-Za-z0-9_-]{1,64}$/)
            expect(id).not.toBe('not a valid id!')
        })
    })

    describe('getDeviceName', () => {
        it.each([
            [
                'Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0',
                'Firefox on Linux'
            ],
            [
                'Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36',
                'Chrome on Android'
            ],
            [
                'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15',
                'Safari on macOS'
            ],
            [
                'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0',
                'Edge on Windows'
            ],
            [
                'Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1',
                'Safari on iOS'
            ]
        ])('labels %s as %s', async (ua, want) => {
            stubUserAgent(ua)
            const { getDeviceName } = await freshModule()
            expect(getDeviceName()).toBe(want)
        })

        // An unrecognised agent still needs a label the user can click Revoke on.
        it('falls back to a generic label for an unknown agent', async () => {
            stubUserAgent('some-unknown-agent/1.0')
            const { getDeviceName } = await freshModule()
            expect(getDeviceName()).toBe('Aether')
        })
    })
})
