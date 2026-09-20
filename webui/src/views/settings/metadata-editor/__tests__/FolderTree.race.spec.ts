import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import FolderTree from '@/views/settings/metadata-editor/FolderTree.vue'
import type { Folder } from '@/types/metadata'

// Drive listFolders/searchFolders through hand-resolved promises so we can land
// responses out of order and prove a stale one can't overwrite fresh state.
const listFolders = vi.hoisted(() => vi.fn())
const searchFolders = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api/Metadata', () => ({ listFolders, searchFolders }))

const f = (name: string): Folder => ({ name, path: name, has_subfolders: false })

// A deferred promise whose resolution we control from the test.
function deferred<T>() {
    let resolve!: (v: T) => void
    const promise = new Promise<T>((r) => (resolve = r))
    return { promise, resolve }
}

beforeEach(() => {
    listFolders.mockReset()
    searchFolders.mockReset()
})

const TreeStub = {
    name: 'Tree',
    props: ['value', 'expandedKeys', 'selectionKeys'],
    emits: ['node-expand', 'node-select', 'update:selectionKeys'],
    template: '<div />'
}

const tree = (w: ReturnType<typeof mount>) => w.findComponent(TreeStub)
const labels = (w: ReturnType<typeof mount>) =>
    (tree(w).props('value') as { label: string }[]).map((n) => n.label)

describe('FolderTree stale-response races', () => {
    it('discards a slow root load from the previous scan folder', async () => {
        const libA = deferred<Folder[]>()
        const libB = deferred<Folder[]>()
        listFolders.mockImplementation((name: string) =>
            name === 'A' ? libA.promise : libB.promise
        )

        const w = mount(FolderTree, {
            props: { scanFolder: 'A' },
            global: { stubs: { Tree: TreeStub } }
        })
        // Switch to scan folder B while A's root load is still in flight.
        await w.setProps({ scanFolder: 'B' })

        // Scan folder B lands first and paints its folders.
        libB.resolve([f('OnlyInB')])
        await flushPromises()
        expect(labels(w)).toEqual(['OnlyInB'])

        // The stale scan-folder-A response arrives afterwards and must be dropped.
        libA.resolve([f('OnlyInA')])
        await flushPromises()
        expect(labels(w)).toEqual(['OnlyInB'])
    })

    it('discards a slow search result from a superseded query', async () => {
        listFolders.mockResolvedValue([])
        const first = deferred<{ folders: Folder[]; truncated: boolean }>()
        const second = deferred<{ folders: Folder[]; truncated: boolean }>()
        searchFolders
            .mockImplementationOnce(() => first.promise)
            .mockImplementationOnce(() => second.promise)

        const w = mount(FolderTree, {
            props: { scanFolder: 'Main', filter: 'foo' },
            global: { stubs: { Tree: TreeStub } }
        })
        await flushPromises()
        // A second query supersedes the first.
        await w.setProps({ filter: 'bar' })

        // The newer query resolves first.
        second.resolve({ folders: [f('BarMatch')], truncated: false })
        await flushPromises()
        expect(labels(w)).toEqual(['BarMatch'])

        // The stale first query resolves last and must not overwrite the results.
        first.resolve({ folders: [f('FooMatch')], truncated: false })
        await flushPromises()
        expect(labels(w)).toEqual(['BarMatch'])
    })
})
