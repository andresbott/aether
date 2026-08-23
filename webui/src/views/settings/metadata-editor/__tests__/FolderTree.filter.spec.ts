import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import FolderTree from '@/views/settings/metadata-editor/FolderTree.vue'
import type { Folder } from '@/types/metadata'

// Filter mode calls searchFolders and renders the filtered branches; normal mode
// still lazy-loads via listFolders. Mock both.
const listFolders = vi.hoisted(() => vi.fn())
const searchFolders = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api/Metadata', () => ({ listFolders, searchFolders }))

const f = (name: string, path: string, hasSub = false): Folder => ({
    name,
    path,
    has_subfolders: hasSub
})

beforeEach(() => {
    listFolders.mockReset()
    listFolders.mockResolvedValue([f('Alexia dixon', 'Alexia dixon', true), f('Other', 'Other', true)])
    searchFolders.mockReset()
    searchFolders.mockResolvedValue({
        folders: [f('fire up', 'Alexia dixon/fire up', false)],
        truncated: false
    })
})

const TreeStub = {
    name: 'Tree',
    props: ['value', 'expandedKeys', 'selectionKeys'],
    emits: ['node-expand', 'node-select', 'update:selectionKeys'],
    template: '<div />'
}

async function mountTree(filter?: string) {
    const w = mount(FolderTree, {
        props: { libraryId: 1, filter },
        global: { stubs: { Tree: TreeStub } }
    })
    await flushPromises()
    return w
}
const tree = (w: ReturnType<typeof mount>) => w.findComponent(TreeStub)

describe('FolderTree filter mode', () => {
    it('shows only the branches leading to a match, fully expanded', async () => {
        const w = await mountTree('up')
        expect(searchFolders).toHaveBeenCalledWith(1, 'up')
        const nodes = tree(w).props('value') as any[]
        expect(nodes).toHaveLength(1)
        expect(nodes[0].key).toBe('Alexia dixon')
        expect(nodes[0].children[0].key).toBe('Alexia dixon/fire up')
        expect(tree(w).props('expandedKeys')).toEqual({ 'Alexia dixon': true })
    })

    it('emits the folder path when a filtered node is selected', async () => {
        const w = await mountTree('up')
        const child = (tree(w).props('value') as any[])[0].children[0]
        tree(w).vm.$emit('node-select', child)
        expect(w.emitted('select')?.[0]).toEqual(['Alexia dixon/fire up'])
    })

    it('restores the lazy tree when the filter is cleared', async () => {
        const w = await mountTree('up')
        await w.setProps({ filter: '' })
        await flushPromises()
        const nodes = tree(w).props('value') as any[]
        expect(nodes.map((n) => n.key)).toEqual(['Alexia dixon', 'Other'])
        expect(searchFolders).toHaveBeenCalledTimes(1) // no search once cleared
    })
})
