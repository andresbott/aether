import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import type { ServerVersion } from '@/types/version'

const versionData = ref<ServerVersion | undefined>(undefined)
vi.mock('@/composables/useVersion', () => ({
    useVersion: () => ({ data: versionData })
}))

const isTouch = ref(false)
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({
        tier: ref('desktop'),
        shell: ref('desktop'),
        isTouch
    })
}))

// AboutView renders ContentScaffold, which calls useRouter().
const push = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ push })
}))

// A browser tab unless a case says otherwise.
const host = vi.hoisted(() => ({
    desktopApp: false,
    appVersion: vi.fn<() => Promise<string | null>>()
}))
vi.mock('@/lib/host', async () => {
    const { ref } = await import('vue')
    return {
        isDesktopApp: () => host.desktopApp,
        serverUrl: ref<string | null>(null),
        appVersion: host.appVersion
    }
})

import AboutView from '@/views/AboutView.vue'
import { serverUrl } from '@/lib/host'

const mountView = () => mount(AboutView)

beforeEach(() => {
    versionData.value = undefined
    isTouch.value = false
    host.desktopApp = false
    host.appVersion.mockReset()
    serverUrl.value = null
})

describe('AboutView', () => {
    it('renders as a main content view with the scaffold title', () => {
        const w = mountView()
        expect(w.find('.content-scaffold').exists()).toBe(true)
        expect(w.find('.scaffold-title h1').text()).toBe('About')
    })

    it('orders the sections shortcuts, build, source, credits with no brand hero', () => {
        const w = mountView()
        const headers = w.findAll('.about-section h2').map((h) => h.text())
        expect(headers).toEqual(['Keyboard shortcuts', 'Build', 'Source', 'Credits'])
        expect(w.find('.about-hero').exists()).toBe(false)
    })

    it('shows the build info with the v prefix on numeric versions', () => {
        versionData.value = {
            version: '0.1.1',
            commit: 'abcdef1234567890',
            build_time: '2026-07-25T10:00:00Z'
        }
        const text = mountView().text()
        expect(text).toContain('v0.1.1')
        expect(text).toContain('abcdef12')
        expect(text).toContain('2026-07-25T10:00:00Z')
    })

    it('leaves non-release build names unprefixed and hides empty fields', () => {
        versionData.value = { version: 'dev-build', commit: 'undefined', build_time: '' }
        const w = mountView()
        expect(w.text()).toContain('dev-build')
        // "undefined" commits and empty build times are placeholder junk, not info.
        expect(w.findAll('.info-row')).toHaveLength(1)
    })

    it('reports the version as unknown while it has not loaded', () => {
        expect(mountView().text()).toContain('unknown')
    })

    it('links the source repository', () => {
        const hrefs = mountView()
            .findAll('a')
            .map((a) => a.attributes('href'))
        expect(hrefs).toContain('https://github.com/andresbott/aether')
    })

    it('notes that shortcuts need a keyboard on touch devices', () => {
        isTouch.value = true
        expect(mountView().text()).toContain('Keyboard shortcuts apply when a keyboard is attached.')
    })
})

// The Build section as the user reads it: [label, value] per row.
const buildRows = (w: ReturnType<typeof mountView>) =>
    w.findAll('.info-row').map((row) => [row.find('dt').text(), row.find('dd').text()])

const serverBuild = {
    version: '0.1.1',
    commit: 'abcdef1234567890',
    build_time: '2026-07-25T10:00:00Z'
}

describe('AboutView build section in a browser tab', () => {
    // The server is the page's own origin there and there is no app: nothing to
    // tell apart, so the section reads as it always did.
    it('lists the version, commit and build time, with no Server or App row', () => {
        versionData.value = serverBuild
        serverUrl.value = 'http://localhost:8075'

        expect(buildRows(mountView())).toEqual([
            ['Version', 'v0.1.1'],
            ['Commit', 'abcdef12'],
            ['Built', '2026-07-25T10:00:00Z']
        ])
        expect(host.appVersion).not.toHaveBeenCalled()
    })
})

describe('AboutView build section in the desktop app', () => {
    beforeEach(() => {
        host.desktopApp = true
        host.appVersion.mockResolvedValue('0.3.1')
        serverUrl.value = 'https://music.example.com:8443/aether'
        versionData.value = serverBuild
    })

    it('lists the app, the full server address and the server version', async () => {
        const w = mountView()
        await flushPromises()

        expect(buildRows(w)).toEqual([
            ['App', 'v0.3.1'],
            ['Server', 'https://music.example.com:8443/aether'],
            ['Server version', 'v0.1.1'],
            ['Commit', 'abcdef12'],
            ['Built', '2026-07-25T10:00:00Z']
        ])
    })

    it('reads unknown until the player answers, then shows its version', async () => {
        let answer!: (version: string) => void
        host.appVersion.mockReturnValue(new Promise((resolve) => (answer = resolve)))
        const w = mountView()
        expect(buildRows(w)[0]).toEqual(['App', 'unknown'])

        answer('0.3.1')
        await flushPromises()
        expect(buildRows(w)[0]).toEqual(['App', 'v0.3.1'])
    })

    it('leaves a non-release app version unprefixed, as for the server', async () => {
        host.appVersion.mockResolvedValue('dev-build')
        const w = mountView()
        await flushPromises()
        expect(buildRows(w)[0]).toEqual(['App', 'dev-build'])
    })

    it('reads unknown when the player has no version to give', async () => {
        host.appVersion.mockResolvedValue(null)
        const w = mountView()
        await flushPromises()
        expect(buildRows(w)[0]).toEqual(['App', 'unknown'])
    })

    it('reads unknown, and keeps the rest of the page, when asking the player fails', async () => {
        host.appVersion.mockRejectedValue(new Error('ipc unavailable'))
        const w = mountView()
        await flushPromises()

        expect(buildRows(w)).toEqual([
            ['App', 'unknown'],
            ['Server', 'https://music.example.com:8443/aether'],
            ['Server version', 'v0.1.1'],
            ['Commit', 'abcdef12'],
            ['Built', '2026-07-25T10:00:00Z']
        ])
        expect(w.find('.scaffold-title h1').text()).toBe('About')
    })

    it('asks the player once, however often the page redraws', async () => {
        const w = mountView()
        await flushPromises()

        versionData.value = { ...serverBuild, version: '0.2.0' }
        await nextTick()

        expect(buildRows(w)[2]).toEqual(['Server version', 'v0.2.0'])
        expect(host.appVersion).toHaveBeenCalledOnce()
    })
})
