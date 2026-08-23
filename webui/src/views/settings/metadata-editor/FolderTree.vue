<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Tree from 'primevue/tree'
import type { TreeExpandedKeys } from 'primevue/tree'
import type { TreeNode } from 'primevue/treenode'
import { listFolders, searchFolders } from '@/lib/api/Metadata'
import { apiErrorMessage } from '@/lib/apiError'
import { buildFilteredFolderTree } from './folderFilter'

const props = defineProps<{
    libraryId: number | null
    expandTo?: string | null
    // A non-blank filter switches the tree to server-side search mode.
    filter?: string | null
}>()
const emit = defineEmits<{
    (e: 'select', path: string): void
}>()

const nodes = ref<TreeNode[]>([])
const expandedKeys = ref<TreeExpandedKeys>({})
const selectionKeys = ref<Record<string, boolean>>({})
const loadError = ref<string | null>(null)

// ----- Filter (search) mode -----
// A non-blank `filter` switches from lazy per-level browsing to a flat
// server-side search: searchFolders returns every folder whose name matches, and
// buildFilteredFolderTree turns those paths into a tree of just the matching
// branches, fully expanded, so a deep folder is reachable without expanding to
// it. Clearing the filter falls straight back to the lazy `nodes`.
const filterQuery = computed(() => (props.filter ?? '').trim())
const filtering = computed(() => filterQuery.value !== '' && props.libraryId !== null)
const filteredNodes = ref<TreeNode[]>([])
const filteredExpandedKeys = ref<TreeExpandedKeys>({})
const searching = ref(false)
const searchTruncated = ref(false)

const displayNodes = computed(() => (filtering.value ? filteredNodes.value : nodes.value))
const displayExpandedKeys = computed(() =>
    filtering.value ? filteredExpandedKeys.value : expandedKeys.value
)

async function runSearch() {
    if (!filtering.value) {
        filteredNodes.value = []
        filteredExpandedKeys.value = {}
        searchTruncated.value = false
        return
    }
    searching.value = true
    loadError.value = null
    try {
        const { folders, truncated } = await searchFolders(
            props.libraryId as number,
            filterQuery.value
        )
        const built = buildFilteredFolderTree(folders)
        filteredNodes.value = built.nodes
        filteredExpandedKeys.value = built.expandedKeys
        searchTruncated.value = truncated
    } catch (err: any) {
        loadError.value = apiErrorMessage(err)
        filteredNodes.value = []
        filteredExpandedKeys.value = {}
        searchTruncated.value = false
    } finally {
        searching.value = false
    }
}

watch([filterQuery, () => props.libraryId], runSearch, { immediate: true })

function makeNode(name: string, path: string, leaf: boolean): TreeNode {
    return {
        key: path || '__root__',
        label: name,
        icon: leaf ? 'pi pi-folder' : 'pi pi-folder-open',
        leaf,
        data: { path },
        children: leaf ? undefined : []
    }
}

async function loadChildren(parentPath: string): Promise<TreeNode[]> {
    if (props.libraryId === null) return []
    const folders = await listFolders(props.libraryId, parentPath)
    return folders.map((f) =>
        makeNode(
            f.name,
            parentPath === '' ? f.path : `${parentPath}/${f.path}`,
            !f.has_subfolders
        )
    )
}

// expandToPath opens the tree down to `target` (a full relative path), lazily
// loading each level, so the folder picker can jump straight to the breadcrumb
// segment the user clicked. Every ancestor and the target itself are expanded,
// but nothing is selected: a pre-selected node would need two clicks to load
// (PrimeVue single-select toggles the selected node off first), so we leave
// selection to the user's click. If a segment no longer exists on disk it stops
// at the deepest folder that does — expanding what it reached.
async function expandToPath(target: string) {
    if (!target || props.libraryId === null) return
    const parts = target.split('/')
    let level = nodes.value
    let acc = ''
    const nextExpanded: TreeExpandedKeys = {}
    for (let i = 0; i < parts.length; i++) {
        acc = acc ? `${acc}/${parts[i]}` : parts[i]
        const node = level.find((n) => n.data.path === acc)
        if (!node) break
        if (node.leaf) break
        if (!node.children || node.children.length === 0) {
            node.children = await loadChildren(node.data.path)
        }
        nextExpanded[node.key as string] = true
        level = node.children ?? []
    }
    expandedKeys.value = nextExpanded
}

async function resetTree() {
    nodes.value = []
    expandedKeys.value = {}
    selectionKeys.value = {}
    loadError.value = null
    if (props.libraryId === null) return
    try {
        nodes.value = await loadChildren('')
        if (props.expandTo) await expandToPath(props.expandTo)
    } catch (err: any) {
        loadError.value = apiErrorMessage(err)
    }
}

async function onNodeExpand(node: TreeNode) {
    if (node.children && node.children.length > 0) return
    try {
        node.children = await loadChildren(node.data.path)
    } catch (err: any) {
        loadError.value = apiErrorMessage(err)
    }
}

function onNodeSelect(node: TreeNode) {
    emit('select', node.data.path)
}

watch(() => props.libraryId, resetTree, { immediate: true })

// A new target while the tree is already mounted (the picker stays open and the
// user picks another breadcrumb) re-walks without reloading the root.
watch(
    () => props.expandTo,
    (target) => {
        if (target) expandToPath(target).catch((err) => (loadError.value = apiErrorMessage(err)))
    }
)
</script>

<template>
    <div class="folder-tree">
        <div v-if="loadError" class="error-banner">{{ loadError }}</div>
        <div v-if="filtering && searchTruncated && !loadError" class="truncated-hint">
            Showing the first matches — refine your search to narrow it down.
        </div>
        <Tree
            :value="displayNodes"
            :expandedKeys="displayExpandedKeys"
            selectionMode="single"
            v-model:selectionKeys="selectionKeys"
            @node-expand="onNodeExpand"
            @node-select="onNodeSelect"
        />
        <div v-if="libraryId === null" class="empty">Pick a library above.</div>
        <div
            v-else-if="filtering && searching && filteredNodes.length === 0"
            class="empty"
        >
            Searching…
        </div>
        <div
            v-else-if="filtering && !searching && filteredNodes.length === 0 && !loadError"
            class="empty"
        >
            No folders match “{{ filterQuery }}”.
        </div>
        <div v-else-if="!filtering && !loadError && nodes.length === 0" class="empty">
            Loading…
        </div>
    </div>
</template>

<style scoped>
.folder-tree {
    border: 1px solid var(--app-border);
    border-radius: 6px;
    padding: 0.5rem;
    /* Fill the picker's tree column and scroll internally, so a big folder list
       stays inside the dialog instead of stretching it past the viewport. The
       min-height is the floor when the column is unbounded (phone: stacked). */
    height: 100%;
    min-height: 300px;
    overflow: auto;
    box-sizing: border-box;
    background: var(--app-surface);
}
.error-banner {
    background: var(--p-red-50, #fee2e2);
    color: var(--p-red-700, #b91c1c);
    padding: 0.5rem 0.75rem;
    border-radius: 4px;
    margin-bottom: 0.5rem;
    font-size: 0.85rem;
}
.empty {
    padding: 1rem;
    color: var(--app-text-secondary);
    font-size: 0.9rem;
}
.truncated-hint {
    padding: 0.4rem 0.5rem;
    margin-bottom: 0.5rem;
    color: var(--app-text-secondary);
    font-size: 0.8rem;
    font-style: italic;
}
</style>
