<script setup lang="ts">
import { computed, ref } from 'vue'
import Button from 'primevue/button'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import { useConfirm } from 'primevue/useconfirm'
import ConfirmDialog from 'primevue/confirmdialog'
import {
    useLibraries,
    useCreateLibrary,
    useUpdateLibrary,
    useDeleteLibrary
} from '@/composables/useLibraries'
import type { Library, LibraryInput } from '@/types/libraries'
import LibraryDialog from './LibraryDialog.vue'
import { useViewport } from '@/composables/useViewport'

const { data: libraries, isLoading } = useLibraries()
const createMutation = useCreateLibrary()
const updateMutation = useUpdateLibrary()
const deleteMutation = useDeleteLibrary()
const confirm = useConfirm()

const dialogVisible = ref(false)
const editing = ref<Library | null>(null)

function openCreate() {
    editing.value = null
    dialogVisible.value = true
}
function openEdit(lib: Library) {
    editing.value = lib
    dialogVisible.value = true
}
function onSubmit(input: LibraryInput) {
    if (editing.value) {
        updateMutation.mutate(
            { id: editing.value.id, input },
            { onSuccess: () => (dialogVisible.value = false) }
        )
    } else {
        createMutation.mutate(input, {
            onSuccess: () => (dialogVisible.value = false)
        })
    }
}
function onDelete(lib: Library) {
    confirm.require({
        message: `Delete library "${lib.name}" (${lib.path})? This will remove ${lib.track_count} tracks and any related stars/play history. This cannot be undone.`,
        header: 'Delete library?',
        icon: 'pi pi-exclamation-triangle',
        rejectLabel: 'Cancel',
        acceptLabel: 'Delete',
        acceptClass: 'p-button-danger',
        accept: () => deleteMutation.mutate(lib.id)
    })
}

// Libraries declared in the server's config file are read-only here: the server
// rewrites them from the file on every startup, so the API refuses edits (409
// config_managed) rather than accepting a change that would silently revert.
function isConfigManaged(lib: Library): boolean {
    return lib.source === 'config'
}

const configManagedHint =
    'Declared in the server config file. Edit the Libraries section of config.yaml and restart to change it.'

function formatDate(s: string | null): string {
    if (!s) return '—'
    return new Date(s).toLocaleString()
}

const submitting = computed(
    () => createMutation.isPending.value || updateMutation.isPending.value
)

const { tier } = useViewport()
// Spec §5: settings tables must not overflow a phone; these columns are the
// ones a phone admin can live without (the row's dialog still shows all).
const phoneCols = computed(() => tier.value === 'phone')
</script>

<template>
    <section class="section">
        <div class="section-header">
            <h2>Libraries</h2>
            <Button label="Add library" icon="pi pi-plus" @click="openCreate" />
        </div>

        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 1.5rem"></i>
        </div>

        <div v-else-if="!libraries || libraries.length === 0" class="empty-state">
            <p>No libraries configured yet.</p>
            <p>Add a library to start scanning music.</p>
        </div>

        <div v-else class="table-fit">
            <DataTable :value="libraries" responsiveLayout="scroll">
                <Column field="name" header="Name">
                    <template #body="{ data }">
                        <span class="library-name">
                            <i :class="`pi pi-${data.icon || 'folder'}`"></i>
                            {{ data.name }}
                            <Tag
                                v-if="isConfigManaged(data)"
                                class="config-badge"
                                value="From config"
                                severity="secondary"
                                v-tooltip.top="configManagedHint"
                            />
                        </span>
                    </template>
                </Column>
                <Column field="path" header="Path" />
                <Column
                    field="track_count"
                    header="Tracks"
                    :hidden="phoneCols"
                    style="width: 7rem; text-align: right"
                />
                <Column header="Last scan" :hidden="phoneCols" style="width: 14rem">
                    <template #body="{ data }">{{ formatDate(data.last_scan_started_at) }}</template>
                </Column>
                <Column header="" style="width: 11rem; text-align: right">
                    <template #body="{ data }">
                        <!-- Config-provisioned libraries have no actions: the server
                             rewrites them from config.yaml on every startup, so an
                             edit here would be reverted on the next restart. -->
                        <span v-if="isConfigManaged(data)" class="config-hint">
                            Managed in config.yaml
                        </span>
                        <template v-else>
                            <Button
                                icon="pi pi-pencil"
                                text
                                rounded
                                @click="openEdit(data)"
                            />
                            <Button
                                icon="pi pi-trash"
                                text
                                rounded
                                severity="danger"
                                @click="onDelete(data)"
                            />
                        </template>
                    </template>
                </Column>
            </DataTable>
        </div>

        <LibraryDialog
            v-model:visible="dialogVisible"
            :library="editing"
            :submitting="submitting"
            @submit="onSubmit"
            @cancel="dialogVisible = false"
        />

        <ConfirmDialog />
    </section>
</template>

<style scoped>
.section {
    margin-bottom: 2.5rem;
}
.section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 1rem;
}
.section h2 {
    font-size: 1.25rem;
    font-weight: 600;
    margin: 0;
}
.loading {
    display: flex;
    justify-content: center;
    padding: 2rem;
    color: var(--app-text-secondary);
}
.empty-state {
    text-align: center;
    padding: 2rem;
    color: var(--app-text-secondary);
}
.library-name {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
}
.config-badge {
    font-size: 0.75rem;
}
.config-hint {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
}
.table-fit {
    overflow-x: auto;
}
</style>
