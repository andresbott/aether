import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import type { ServerVersion } from '@/types/version'

const versionData = ref<ServerVersion | undefined>(undefined)
vi.mock('@/composables/useVersion', () => ({
    useVersion: () => ({ data: versionData })
}))

import AboutView from '@/views/AboutView.vue'

const mountView = () => mount(AboutView)

beforeEach(() => {
    versionData.value = undefined
})

describe('AboutView', () => {
    it('renders as a main content view with the scaffold title', () => {
        const w = mountView()
        expect(w.find('.content-scaffold').exists()).toBe(true)
        expect(w.find('.scaffold-title h1').text()).toBe('About')
    })

    it('orders the sections shortcuts, build, source with no brand hero', () => {
        const w = mountView()
        const headers = w.findAll('.about-section h2').map((h) => h.text())
        expect(headers).toEqual(['Keyboard shortcuts', 'Build', 'Source'])
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
})
