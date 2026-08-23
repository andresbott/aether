import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import FolderTree from '@/views/settings/metadata-editor/FolderTree.vue'
import type { Folder } from '@/types/metadata'

// FolderTree lazy-loads each folder level through listFolders; drive it from a
// fixed in-memory hierarchy so we can assert exactly which levels it fetched.
const listFolders = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api/Metadata', () => ({ listFolders }))

const f = (name: string, hasSub: boolean): Folder => ({ name, path: name, has_subfolders: hasSub })

// Artist/ -> Album/ -> Disc 1 (leaf), plus a root-level leaf.
const byParent: Record<string, Folder[]> = {
    '': [f('Artist', true), f('Solo', false)],
    Artist: [f('Album', true)],
    'Artist/Album': [f('Disc 1', false)]
}

beforeEach(() => {
    listFolders.mockReset()
    listFolders.mockImplementation((_id: number, parent: string) =>
        Promise.resolve(byParent[parent] ?? [])
    )
})

// Stub the PrimeVue Tree so we can read the expanded/selection state FolderTree
// drives onto it without rendering the real widget.
const TreeStub = {
    name: 'Tree',
    props: ['value', 'expandedKeys', 'selectionKeys'],
    emits: ['node-expand', 'node-select', 'update:selectionKeys'],
    template: '<div />'
}

async function mountTree(expandTo?: string | null) {
    const w = mount(FolderTree, {
        props: { libraryId: 1, expandTo },
        global: { stubs: { Tree: TreeStub } }
    })
    await flushPromises()
    return w
}

const tree = (w: ReturnType<typeof mount>) => w.findComponent(TreeStub)

describe('FolderTree expand-to-path', () => {
    it('expands every ancestor and the target folder without selecting anything', async () => {
        const w = await mountTree('Artist/Album')
        expect(listFolders).toHaveBeenCalledWith(1, '')
        expect(listFolders).toHaveBeenCalledWith(1, 'Artist')
        expect(listFolders).toHaveBeenCalledWith(1, 'Artist/Album')
        expect(tree(w).props('expandedKeys')).toEqual({ Artist: true, 'Artist/Album': true })
        // The target must NOT be pre-selected: PrimeVue single-select toggles the
        // selected node OFF on first click, so a pre-selected target needs two
        // clicks to load. Opening unselected means one click selects and loads.
        expect(tree(w).props('selectionKeys')).toEqual({})
    })

    it('stops at the deepest folder that still exists and selects nothing', async () => {
        const w = await mountTree('Artist/Ghost')
        expect(listFolders).toHaveBeenCalledWith(1, 'Artist')
        expect(listFolders).not.toHaveBeenCalledWith(1, 'Artist/Ghost')
        expect(tree(w).props('expandedKeys')).toEqual({ Artist: true })
        expect(tree(w).props('selectionKeys')).toEqual({})
    })

    it('loads only the root and expands nothing without a target', async () => {
        const w = await mountTree()
        expect(listFolders).toHaveBeenCalledTimes(1)
        expect(listFolders).toHaveBeenCalledWith(1, '')
        expect(tree(w).props('expandedKeys')).toEqual({})
        expect(tree(w).props('selectionKeys')).toEqual({})
    })
})
