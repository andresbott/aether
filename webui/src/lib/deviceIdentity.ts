// This app instance's identity for the /rest credential lifecycle. A user may
// be signed in from several first-party Aether apps at once — browsers today,
// native apps later — and the server keys one session token per device id, so
// this id is what keeps one app signing in from signing another out
// (docs/agents/authentication.md).
import { loadFromLocalStorage, saveToLocalStorage } from '@/utils/localStorage'

/**
 * localStorage key of the device id. Deliberately survives the logout purge:
 * it identifies the app instance, not the user — a fresh id per login would
 * orphan this app's server-side session on every cycle.
 */
export const DEVICE_ID_KEY = 'aether:deviceId'

/** The server takes the id as a scope segment: [A-Za-z0-9_-], up to 64 chars. */
const DEVICE_ID_PATTERN = /^[A-Za-z0-9_-]{1,64}$/

// 22 base64url characters ≈ 128 bits, well inside the server's 64-char bound.
function generateDeviceId(): string {
    const bytes = new Uint8Array(16)
    crypto.getRandomValues(bytes)
    let binary = ''
    for (const byte of bytes) binary += String.fromCharCode(byte)
    return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

let cached: string | null = null

/**
 * This app instance's stable device id, generated on first use and persisted. A
 * stored value the server would reject (tampered, or from an older scheme) is
 * replaced rather than sent: a 400 on the mint would leave the player blank.
 */
export function getDeviceId(): string {
    if (cached) return cached
    const stored = loadFromLocalStorage<string>(DEVICE_ID_KEY, '')
    if (typeof stored === 'string' && DEVICE_ID_PATTERN.test(stored)) {
        cached = stored
        return cached
    }
    cached = generateDeviceId()
    saveToLocalStorage(DEVICE_ID_KEY, cached)
    return cached
}

// Engine order matters: Edge and Chrome both claim "Chrome", and every iOS
// browser claims Safari's engine, so the more specific token has to win.
const BROWSERS: Array<[string, RegExp]> = [
    ['Edge', /Edg[A-Z]?\//],
    ['Opera', /OPR\/|Opera\//],
    ['Firefox', /Firefox\/|FxiOS\//],
    ['Chrome', /Chrome\/|CriOS\//],
    ['Safari', /Safari\//]
]

const PLATFORMS: Array<[string, RegExp]> = [
    ['Android', /Android/],
    ['iOS', /iPhone|iPad|iPod/],
    ['Windows', /Windows/],
    ['macOS', /Mac OS X|Macintosh/],
    ['Linux', /Linux|X11/]
]

function match(table: Array<[string, RegExp]>, ua: string): string | null {
    for (const [label, pattern] of table) {
        if (pattern.test(ua)) return label
    }
    return null
}

/**
 * A human label for this app instance ("Firefox on Linux"), shown as the
 * session's name in User settings so the user can tell their apps apart.
 * Cosmetic: an unrecognised agent still gets a label worth clicking Revoke on.
 */
export function getDeviceName(): string {
    const ua = navigator.userAgent
    const browser = match(BROWSERS, ua)
    const platform = match(PLATFORMS, ua)
    if (browser && platform) return `${browser} on ${platform}`
    return browser ?? platform ?? 'Aether'
}
