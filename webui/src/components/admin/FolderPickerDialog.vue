<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import Tree from 'primevue/tree'
import Checkbox from 'primevue/checkbox'
import Message from 'primevue/message'
import type { TreeNode } from 'primevue/treenode'
import { browseFolders } from '@/lib/api/Libraries'
import { apiErrorMessage } from '@/lib/apiError'
import type { BrowseFolder } from '@/types/libraries'

// Picks a directory for a library's `path` filter — "tracks whose file lies
// under this directory" — not a library's root: a library is a filtered view
// over the whole catalog, and path is just one of its filter fields. The
// server confines browsing to the configured scan folders: calling
// browseFolders with no path answers their roots, and nothing outside them
// is reachable.
const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    (e: 'select', path: string): void
}>()

const nodes = ref<TreeNode[]>([])
const selectionKeys = ref<Record<string, boolean>>({})
const expandedKeys = ref<Record<string, boolean>>({})
const selectedPath = ref<string | null>(null)
const showHidden = ref(false)
const loadError = ref<string | null>(null)

function makeNode(f: BrowseFolder, parent: TreeNode | null): TreeNode {
    const leaf = !f.has_subfolders
    // A symlinked ancestor makes every folder below it symlinked too: the
    // scanner records tracks under the real location a link points to, so a
    // path filter anywhere below a link is just as broken as one on the link
    // itself. That ancestry is carried on the node so a descendant loaded
    // later — even after a reload — still knows about it.
    const symlinked = f.is_symlink || !!parent?.data?.symlinked
    return {
        key: f.path,
        label: f.name,
        // The scan-folder roots get their own icon; below them, a symlinked
        // directory gets its own icon so an admin can tell what they are
        // picking.
        icon: parent === null ? 'pi pi-database' : f.is_symlink ? 'pi pi-link' : 'pi pi-folder',
        leaf,
        data: { path: f.path, symlinked },
        children: leaf ? undefined : []
    }
}

// `path` undefined asks the server for the scan-folder roots — the only
// folders this picker may ever start from; `parent` is the node being
// expanded (null at the roots), passed through to `makeNode` for symlink
// ancestry.
async function loadChildren(
    path: string | undefined,
    parent: TreeNode | null = null
): Promise<TreeNode[]> {
    const res = await browseFolders(path, showHidden.value)
    return res.folders.map((f) => makeNode(f, parent))
}

function findNode(list: TreeNode[], key: string): TreeNode | null {
    for (const n of list) {
        if (n.key === key) return n
        const hit = n.children ? findNode(n.children, key) : null
        if (hit) return hit
    }
    return null
}

// The node currently backing `selectedPath`, looked up live in `nodes` so it
// always reflects the tree's current shape (e.g. after a reload) rather than
// a snapshot taken at selection time.
const selectedNode = computed(() =>
    selectedPath.value ? findNode(nodes.value, selectedPath.value) : null
)
const selectedSymlinked = computed(() => !!selectedNode.value?.data?.symlinked)

async function resetTree() {
    nodes.value = []
    selectionKeys.value = {}
    expandedKeys.value = {}
    selectedPath.value = null
    showHidden.value = false
    loadError.value = null
    try {
        nodes.value = await loadChildren(undefined)
    } catch (err: any) {
        loadError.value = apiErrorMessage(err)
    }
}

// Toggling "show hidden" changes what every already-fetched level contains, so
// the tree is rebuilt from the roots and the open branches are re-fetched.
async function reloadTree() {
    const wasExpanded = Object.keys(expandedKeys.value).filter((k) => expandedKeys.value[k])
    loadError.value = null
    try {
        nodes.value = await loadChildren(undefined)
        // Shallowest first: a path is always shorter than the paths below it, so
        // this guarantees a node's parent is loaded before we look the node up.
        for (const key of wasExpanded.sort((a, b) => a.length - b.length)) {
            const node = findNode(nodes.value, key)
            if (!node || node.leaf) continue
            node.children = await loadChildren(node.data.path, node)
        }
    } catch (err: any) {
        loadError.value = apiErrorMessage(err)
    }
    // Drop a selection the filter no longer shows, so Select can't confirm a
    // path that has just disappeared from the tree.
    if (!showHidden.value && isHiddenPath(selectedPath.value)) {
        selectedPath.value = null
        selectionKeys.value = {}
    }
}

function isHiddenPath(path: string | null): boolean {
    return !!path && path.split('/').some((seg) => seg.startsWith('.'))
}

async function onNodeExpand(node: TreeNode) {
    if (node.children && node.children.length > 0) return
    try {
        node.children = await loadChildren(node.data.path, node)
    } catch (err: any) {
        loadError.value = apiErrorMessage(err)
    }
}

function onNodeSelect(node: TreeNode) {
    selectedPath.value = node.data.path
}

function onConfirm() {
    if (!selectedPath.value) return
    emit('select', selectedPath.value)
    emit('update:visible', false)
}

watch(
    () => props.visible,
    (v) => {
        if (v) resetTree()
    },
    { immediate: true }
)
</script>

<template>
    <Dialog
        :visible="visible"
        @update:visible="emit('update:visible', $event)"
        modal
        header="Select folder"
        :style="{ width: '28rem' }"
    >
        <div v-if="loadError" class="error-banner">{{ loadError }}</div>
        <div class="tree-wrap">
            <Tree
                :value="nodes"
                selectionMode="single"
                v-model:selectionKeys="selectionKeys"
                v-model:expandedKeys="expandedKeys"
                @node-expand="onNodeExpand"
                @node-select="onNodeSelect"
            />
        </div>
        <div class="hidden-toggle">
            <Checkbox
                v-model="showHidden"
                :binary="true"
                inputId="folder-picker-show-hidden"
                data-testid="folder-picker-show-hidden"
                @update:modelValue="reloadTree"
            />
            <label for="folder-picker-show-hidden">Show hidden folders</label>
        </div>
        <div class="selected-path">
            {{ selectedPath ?? 'No folder selected' }}
        </div>
        <Message
            v-if="selectedSymlinked"
            severity="warn"
            :closable="false"
            class="symlink-warning"
            data-testid="folder-picker-symlink-warning"
        >
            A library path filter on or below a symbolic link matches nothing today: tracks are
            recorded under the real location the link points to. Pick the real folder instead.
        </Message>
        <template #footer>
            <Button label="Cancel" text @click="emit('update:visible', false)" />
            <Button
                label="Select"
                data-testid="folder-picker-select"
                :disabled="!selectedPath"
                @click="onConfirm"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.tree-wrap {
    max-height: 40vh;
    overflow-y: auto;
    border: 1px solid var(--app-border);
    border-radius: 6px;
}
/* Symlinked folders read as folders, just marked: the pi-link icon is the
   signal, dimmed so it reads as a qualifier rather than a different kind of
   row. Keyed off the icon class because PrimeVue's TreeNode ignores
   `styleClass` on a node. */
.tree-wrap :deep(.p-tree-node-icon.pi-link) {
    color: var(--app-text-secondary);
}
.hidden-toggle {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin-top: 0.6rem;
    font-size: 0.85rem;
}
.hidden-toggle label {
    cursor: pointer;
}
.selected-path {
    margin-top: 0.75rem;
    font-family: monospace;
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    word-break: break-all;
}
.symlink-warning {
    margin-top: 0.75rem;
}
.error-banner {
    background: var(--p-red-50, #fee2e2);
    color: var(--p-red-700, #b91c1c);
    padding: 0.5rem 0.75rem;
    border-radius: 4px;
    margin-bottom: 0.5rem;
    font-size: 0.85rem;
}
</style>
