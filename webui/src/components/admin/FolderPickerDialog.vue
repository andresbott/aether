<script setup lang="ts">
import { ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import Tree from 'primevue/tree'
import type { TreeNode } from 'primevue/treenode'
import { browseFolders } from '@/lib/api/Libraries'
import { apiErrorMessage } from '@/lib/apiError'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    (e: 'select', path: string): void
}>()

const nodes = ref<TreeNode[]>([])
const selectionKeys = ref<Record<string, boolean>>({})
const selectedPath = ref<string | null>(null)
const loadError = ref<string | null>(null)

function makeNode(name: string, path: string, leaf: boolean): TreeNode {
    return {
        key: path,
        label: name,
        icon: 'pi pi-folder',
        leaf,
        data: { path },
        children: leaf ? undefined : []
    }
}

async function loadChildren(path: string): Promise<TreeNode[]> {
    const res = await browseFolders(path)
    return res.folders.map((f) => makeNode(f.name, f.path, !f.has_subfolders))
}

async function resetTree() {
    nodes.value = []
    selectionKeys.value = {}
    selectedPath.value = null
    loadError.value = null
    try {
        nodes.value = await loadChildren('/')
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
                @node-expand="onNodeExpand"
                @node-select="onNodeSelect"
            />
        </div>
        <div class="selected-path">
            {{ selectedPath ?? 'No folder selected' }}
        </div>
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
.selected-path {
    margin-top: 0.75rem;
    font-family: monospace;
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    word-break: break-all;
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
