import { describe, it, expect } from 'vitest'
import { buildFilteredFolderTree } from '@/views/settings/metadata-editor/folderFilter'
import type { Folder } from '@/types/metadata'

// f builds a search-result folder; the name is the last path segment, like the
// server returns.
const f = (path: string, hasSub = false): Folder => ({
    name: path.split('/').pop() as string,
    path,
    has_subfolders: hasSub
})

describe('buildFilteredFolderTree', () => {
    it('builds the ancestor chain to a deep match and marks only the match', () => {
        const { nodes, expandedKeys } = buildFilteredFolderTree([f('Alexia dixon/fire up')])

        expect(nodes).toHaveLength(1)
        const parent = nodes[0]
        expect(parent.key).toBe('Alexia dixon')
        expect(parent.label).toBe('Alexia dixon')
        expect(parent.data.match).toBe(false) // a synthesized ancestor, not a hit
        expect(parent.leaf).toBe(false)
        expect(parent.children).toHaveLength(1)

        const child = parent.children![0]
        expect(child.key).toBe('Alexia dixon/fire up')
        expect(child.label).toBe('fire up')
        expect(child.data.path).toBe('Alexia dixon/fire up')
        expect(child.data.match).toBe(true)
        expect(child.leaf).toBe(true)

        // The whole matching branch is expanded so the hit is visible at once.
        expect(expandedKeys).toEqual({ 'Alexia dixon': true })
    })

    it('shares one ancestor node across sibling matches', () => {
        const { nodes } = buildFilteredFolderTree([
            f('Alexia dixon/fire up'),
            f('Alexia dixon/ballad')
        ])
        expect(nodes).toHaveLength(1)
        expect(nodes[0].children).toHaveLength(2)
        expect(nodes[0].children!.map((c) => c.label)).toEqual(['ballad', 'fire up'])
    })

    it('marks a matched folder that is also the ancestor of another match', () => {
        const { nodes } = buildFilteredFolderTree([
            f('Alexia dixon', true),
            f('Alexia dixon/fire up')
        ])
        const parent = nodes[0]
        expect(parent.data.match).toBe(true)
        expect(parent.leaf).toBe(false)
        expect(parent.children).toHaveLength(1)
    })

    it('returns an empty tree for no matches', () => {
        expect(buildFilteredFolderTree([])).toEqual({ nodes: [], expandedKeys: {} })
    })

    it('sorts siblings by label regardless of input order', () => {
        const { nodes } = buildFilteredFolderTree([f('Zeta'), f('Alpha')])
        expect(nodes.map((n) => n.label)).toEqual(['Alpha', 'Zeta'])
    })
})
