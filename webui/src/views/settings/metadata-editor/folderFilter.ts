import type { TreeNode } from 'primevue/treenode'
import type { Folder } from '@/types/metadata'

export interface FilteredFolderTree {
    nodes: TreeNode[]
    // Every non-leaf node key, so the caller can open the whole filtered result
    // at once — a filter the user has to expand by hand defeats the point.
    expandedKeys: Record<string, boolean>
}

// buildFilteredFolderTree turns the flat list of matching folders returned by
// searchFolders (each a full library-relative, forward-slash path) into a nested
// tree that shows only the branches leading to a match. Ancestors that did not
// themselves match are synthesized as structural nodes so the match is
// reachable; `data.match` flags the folders that actually matched (for
// highlighting) versus those ancestors. Siblings are sorted by label and every
// node with children is reported in expandedKeys.
export function buildFilteredFolderTree(matches: Folder[]): FilteredFolderTree {
    const matched = new Set(matches.map((m) => m.path))
    const byKey = new Map<string, TreeNode>()
    const roots: TreeNode[] = []

    // ensure returns the node for path, creating it — and every missing
    // ancestor — on the way. A node created only because it lies above a match
    // gets match=false unless the same path is itself in the result set.
    const ensure = (path: string): TreeNode => {
        const existing = byKey.get(path)
        if (existing) return existing
        const slash = path.lastIndexOf('/')
        const node: TreeNode = {
            key: path,
            label: slash === -1 ? path : path.slice(slash + 1),
            data: { path, match: matched.has(path) },
            children: []
        }
        byKey.set(path, node)
        if (slash === -1) {
            roots.push(node)
        } else {
            ensure(path.slice(0, slash)).children!.push(node)
        }
        return node
    }

    for (const m of matches) ensure(m.path)

    const expandedKeys: Record<string, boolean> = {}
    const finalize = (list: TreeNode[]) => {
        list.sort((a, b) => String(a.label).localeCompare(String(b.label)))
        for (const node of list) {
            const children = node.children ?? []
            if (children.length === 0) {
                node.leaf = true
                node.icon = 'pi pi-folder'
                node.children = undefined
            } else {
                node.leaf = false
                node.icon = 'pi pi-folder-open'
                expandedKeys[node.key as string] = true
                finalize(children)
            }
        }
    }
    finalize(roots)

    return { nodes: roots, expandedKeys }
}
