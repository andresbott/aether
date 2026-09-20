import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { ScanFolder } from '@/types/scanFolders'

// The panel destructures useScanFolders()'s return, so the mock must hand back
// real refs — a plain { value } object does not unwrap in the template. Both
// `current` and `loading` are read fresh on every useScanFolders() call (i.e.
// every mount), so a test sets them before mounting.
const state = vi.hoisted(() => ({ current: [] as ScanFolder[], loading: false }))
vi.mock('@/composables/useScanFolders', async () => {
    const { ref: vueRef } = await import('vue')
    return {
        useScanFolders: () => ({ data: vueRef(state.current), isLoading: vueRef(state.loading) })
    }
})

import ScanFoldersPanel from '@/components/admin/ScanFoldersPanel.vue'

// Records what each tooltip binding carried, as a plain attribute — the real
// PrimeVue Tooltip directive stores the value on a JS property, not the DOM,
// so content can't be asserted through it (only presence can). A falsy value
// sets no attribute, mirroring the real directive (which disables the
// tooltip and never sets its own `data-pd-tooltip` marker in that case).
// Mirrors FieldRow.spec.ts's recorder.
const tooltipRecorder = {
    mounted(el: HTMLElement, binding: { value: unknown }) {
        if (binding.value) el.setAttribute('data-tooltip', String(binding.value))
    },
    updated(el: HTMLElement, binding: { value: unknown }) {
        if (binding.value) el.setAttribute('data-tooltip', String(binding.value))
        else el.removeAttribute('data-tooltip')
    }
}

function scanFolder(over: Partial<ScanFolder> = {}): ScanFolder {
    return {
        name: 'Music',
        path: '/mnt/music',
        exclude_patterns: [],
        follow_symlinks: true,
        available: true,
        track_count: 500,
        ...over
    }
}

const mountPanel = (folders: ScanFolder[], loading = false) => {
    state.current = folders
    state.loading = loading
    return mount(ScanFoldersPanel, {
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: tooltipRecorder }
        }
    })
}

describe('ScanFoldersPanel', () => {
    it('shows the heading and the config-file hint regardless of data', async () => {
        const w = mountPanel([scanFolder({})])
        await flushPromises()
        expect(w.text()).toContain('Scan folders')
        expect(w.text()).toContain(
            "Defined in the server's config file under ScanFolders; restart the server to apply changes."
        )
    })

    it('renders a row with its name, path, excludes count, symlink policy and track count', async () => {
        const w = mountPanel([
            scanFolder({
                name: 'Music',
                path: '/mnt/music',
                exclude_patterns: ['*.tmp', '.git'],
                follow_symlinks: false,
                track_count: 4321
            })
        ])
        await flushPromises()
        expect(w.text()).toContain('Music')
        expect(w.text()).toContain('/mnt/music')
        expect(w.text()).toContain('2')
        expect(w.text()).toContain('Not followed')
        expect(w.text()).toContain('4321')
    })

    it('puts the exclude patterns on the Excludes cell as a tooltip', async () => {
        const w = mountPanel([scanFolder({ exclude_patterns: ['*.tmp', '.git'] })])
        await flushPromises()
        const cell = w.find('[data-test="scan-folder-excludes"]')
        expect(cell.text()).toBe('2')
        expect(cell.attributes('data-tooltip')).toBe('*.tmp\n.git')
    })

    it('puts no tooltip on the Excludes cell when there are no patterns', async () => {
        const w = mountPanel([scanFolder({ exclude_patterns: [] })])
        await flushPromises()
        const cell = w.find('[data-test="scan-folder-excludes"]')
        expect(cell.text()).toBe('0')
        expect(cell.attributes('data-tooltip')).toBeUndefined()
    })

    it('shows "Followed" when a folder follows symlinks', async () => {
        const w = mountPanel([scanFolder({ follow_symlinks: true })])
        await flushPromises()
        expect(w.text()).toContain('Followed')
        expect(w.text()).not.toContain('Not followed')
    })

    it('shows a success "Available" tag with no tooltip for a usable folder', async () => {
        const w = mountPanel([scanFolder({ available: true, problem: undefined })])
        await flushPromises()
        const tag = w.find('[data-test="scan-folder-status"]')
        expect(tag.text()).toBe('Available')
        expect(tag.classes()).toContain('p-tag-success')
        expect(tag.attributes('data-tooltip')).toBeUndefined()
    })

    it('shows a danger "Not usable" tag with the problem as its tooltip for an unavailable folder', async () => {
        const w = mountPanel([scanFolder({ available: false, problem: 'root does not exist' })])
        await flushPromises()
        const tag = w.find('[data-test="scan-folder-status"]')
        expect(tag.text()).toBe('Not usable')
        expect(tag.classes()).toContain('p-tag-danger')
        expect(tag.attributes('data-tooltip')).toBe('root does not exist')
    })

    it('shows the empty-state copy when no scan folders are configured', async () => {
        const w = mountPanel([])
        await flushPromises()
        expect(w.text()).toContain(
            "No scan folders are configured — nothing is scanned and no on-disk media is served. Add them under ScanFolders in the server's config file and restart."
        )
    })

    it('shows a loading indicator while the folders are still loading', () => {
        const w = mountPanel([], true)
        expect(w.find('.pi-spinner').exists()).toBe(true)
    })
})
