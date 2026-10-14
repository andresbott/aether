import { describe, it, expect } from 'vitest'
import { serverHost } from '@/lib/host/serverUrl'

// How the app names its server to the user — the sign-in card's "Server · …"
// line, Change server's question: the address minus its scheme.
describe('serverHost', () => {
    it.each([
        ['http://aether.local:8075', 'aether.local:8075'],
        ['https://music.example.org', 'music.example.org'],
        // a path prefix says nothing about which server it is
        ['https://example.org:8443/media/aether', 'example.org:8443'],
        ['http://[::1]:8075', '[::1]:8075']
    ])('names %j as %j', (url, host) => {
        expect(serverHost(url)).toBe(host)
    })

    // Only a hand-edited settings file holds an address that does not parse:
    // show what it holds rather than nothing.
    it('shows an address that does not parse as it is', () => {
        expect(serverHost('not a url')).toBe('not a url')
    })
})
