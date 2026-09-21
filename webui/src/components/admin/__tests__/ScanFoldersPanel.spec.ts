import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { ScanFolder } from '@/types/scanFolders'

// The panel destructures useScanFolders()'s return, so the mock must hand back
// real refs — a plain { value } object does not unwrap in the template. All
// three of `current`, `loading` and `isError` are read fresh on every
// useScanFolders() call (i.e. every mount), so a test sets them before
// mounting. `current` can be undefined — that's the real TanStack Query shape
// of a failed query (data stays undefined, isLoading goes back to false).
const state = vi.hoisted(() => ({
    current: [] as ScanFolder[] | undefined,
    loading: false,
    isError: false
}))
vi.mock('@/composables/useScanFolders', async () => {
    const { ref: vueRef } = await import('vue')
    return {
        useScanFolders: () => ({
            data: vueRef(state.current),
            isLoading: vueRef(state.loading),
            isError: vueRef(state.isError)
        })
    }
})

// Same shape as useCatalogScan(): refs read fresh on every mount, so a test
// sets the scan's state before mounting.
const scan = vi.hoisted(() => ({
    start: (() => {}) as (...args: unknown[]) => void,
    starting: false,
    running: false,
    progressText: ''
}))
vi.mock('@/composables/useCatalogScan', async () => {
    const { ref: vueRef } = await import('vue')
    return {
        useCatalogScan: () => ({
            start: scan.start,
            starting: vueRef(scan.starting),
            running: vueRef(scan.running),
            progressText: vueRef(scan.progressText)
        })
    }
})

beforeEach(() => {
    scan.start = vi.fn()
    scan.starting = false
    scan.running = false
    scan.progressText = ''
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

const mountPanel = (folders: ScanFolder[] | undefined, loading = false, isError = false) => {
    state.current = folders
    state.loading = loading
    state.isError = isError
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

    // A tooltip is unreachable by touch and by keyboard, and this panel is
    // read-only — there is no row dialog to read the reason in instead.
    it('also renders the problem as text in the row, not only as a tooltip', async () => {
        const w = mountPanel([scanFolder({ available: false, problem: 'root does not exist' })])
        await flushPromises()
        const problem = w.find('[data-test="scan-folder-problem"]')
        expect(problem.exists()).toBe(true)
        expect(problem.text()).toBe('root does not exist')
    })

    it('renders no problem text for a usable folder', async () => {
        const w = mountPanel([scanFolder({ available: true, problem: undefined })])
        await flushPromises()
        expect(w.find('[data-test="scan-folder-problem"]').exists()).toBe(false)
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

// A failed request must never be mistaken for "nothing is configured" — that
// would tell the admin a false thing about the server's own config file.
describe('ScanFoldersPanel load error', () => {
    it('shows an error message instead of the empty state when the request fails', async () => {
        const w = mountPanel(undefined, false, true)
        await flushPromises()
        const err = w.find('[data-test="scan-folders-error"]')
        expect(err.exists()).toBe(true)
        expect(err.text()).toBe(
            'Could not load the scan folders. Check that the server is reachable and reload the page.'
        )
        expect(w.text()).not.toContain('No scan folders are configured')
    })

    it('shows the empty state, not the error, once the folders load empty', async () => {
        const w = mountPanel([])
        await flushPromises()
        expect(w.find('[data-test="scan-folders-error"]').exists()).toBe(false)
        expect(w.text()).toContain('No scan folders are configured')
    })
})

describe('ScanFoldersPanel scan now', () => {
    it('starts a catalog scan from the header button', async () => {
        const w = mountPanel([scanFolder({})])
        await flushPromises()
        const button = w.find('[data-test="scan-now"]')
        expect(button.text()).toBe('Scan now')
        expect(button.attributes('disabled')).toBeUndefined()
        await button.trigger('click')
        expect(scan.start).toHaveBeenCalledTimes(1)
    })

    it('shows the running scan’s progress on the button and holds it disabled', async () => {
        scan.running = true
        scan.progressText = '42% complete'
        const w = mountPanel([scanFolder({})])
        await flushPromises()
        const button = w.find('[data-test="scan-now"]')
        expect(button.text()).toBe('42% complete')
        expect(button.attributes('disabled')).toBeDefined()
    })

    it('holds the button disabled while the trigger is in flight', async () => {
        scan.starting = true
        const w = mountPanel([scanFolder({})])
        await flushPromises()
        expect(w.find('[data-test="scan-now"]').attributes('disabled')).toBeDefined()
    })

    it('disables the button when no scan folders are configured', async () => {
        const w = mountPanel([])
        await flushPromises()
        expect(w.find('[data-test="scan-now"]').attributes('disabled')).toBeDefined()
    })

    // A failed fetch says nothing about the server's config, so it must not
    // pass for "nothing to scan" — same rule as the empty-state copy above.
    it('keeps the button usable when the folders failed to load', async () => {
        const w = mountPanel(undefined, false, true)
        await flushPromises()
        expect(w.find('[data-test="scan-now"]').attributes('disabled')).toBeUndefined()
    })
})
