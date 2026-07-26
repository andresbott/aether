<script setup lang="ts">
import { ref, watch } from 'vue'
import Tree from 'primevue/tree'
import type { TreeExpandedKeys } from 'primevue/tree'
import type { TreeNode } from 'primevue/treenode'
import { listFolders } from '@/lib/api/Metadata'
import { apiErrorMessage } from '@/lib/apiError'

const props = defineProps<{ libraryId: number | null }>()
const emit = defineEmits<{
    (e: 'select', path: string): void
}>()

const nodes = ref<TreeNode[]>([])
const expandedKeys = ref<TreeExpandedKeys>({})
const selectionKeys = ref<Record<string, boolean>>({})
const loadError = ref<string | null>(null)

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

async function resetTree() {
    nodes.value = []
    expandedKeys.value = {}
    selectionKeys.value = {}
    loadError.value = null
    if (props.libraryId === null) return
    try {
        nodes.value = await loadChildren('')
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
</script>

<template>
    <div class="folder-tree">
        <div v-if="loadError" class="error-banner">{{ loadError }}</div>
        <Tree
            :value="nodes"
            :expandedKeys="expandedKeys"
            selectionMode="single"
            v-model:selectionKeys="selectionKeys"
            @node-expand="onNodeExpand"
            @node-select="onNodeSelect"
        />
        <div v-if="!loadError && nodes.length === 0 && libraryId !== null" class="empty">
            Loading…
        </div>
        <div v-if="libraryId === null" class="empty">Pick a library above.</div>
    </div>
</template>

<style scoped>
.folder-tree {
    border: 1px solid var(--app-border);
    border-radius: 6px;
    padding: 0.5rem;
    min-height: 300px;
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
</style>
