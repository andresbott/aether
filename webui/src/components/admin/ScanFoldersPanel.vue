<script setup lang="ts">
import { computed } from 'vue'
import Tag from 'primevue/tag'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import { useScanFolders } from '@/composables/useScanFolders'
import { useViewport } from '@/composables/useViewport'
import type { ScanFolder } from '@/types/scanFolders'

// Read-only: scan folders are declared only in the server's config file (see
// docs/agents/architecture.md#scan-folders-config-only) — there is nothing to
// create, edit or delete here, so this panel has no buttons and no dialogs.
const { data: scanFolders, isLoading, isError } = useScanFolders()

const { tier } = useViewport()
// Spec §5: settings tables must not overflow a phone; the path, excludes and
// symlink policy have no other surface on this read-only panel (no row
// dialog), so only the path follows the name below — excludes/symlinks are
// simply not shown on phone. The path it moves there must wrap (see
// `.name-path`), or the widest row stretches the table and pushes Status off
// the screen.
const phoneCols = computed(() => tier.value === 'phone')

function excludesTooltip(folder: ScanFolder): string | undefined {
    return folder.exclude_patterns.length > 0 ? folder.exclude_patterns.join('\n') : undefined
}
</script>

<template>
    <section class="section">
        <div class="section-header">
            <h2>Scan folders</h2>
        </div>
        <p class="hint">
            Defined in the server's config file under ScanFolders; restart the server to apply
            changes.
        </p>

        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 1.5rem"></i>
        </div>

        <div v-else-if="isError" class="error-state" data-test="scan-folders-error">
            Could not load the scan folders. Check that the server is reachable and reload the
            page.
        </div>

        <div v-else-if="scanFolders && scanFolders.length === 0" class="empty-state">
            <p>
                No scan folders are configured — nothing is scanned and no on-disk media is
                served. Add them under ScanFolders in the server's config file and restart.
            </p>
        </div>

        <div v-else class="table-fit">
            <DataTable :value="scanFolders" responsiveLayout="scroll">
                <Column field="name" header="Name">
                    <template #body="{ data }">
                        <span class="folder-name">{{ data.name }}</span>
                        <span v-if="phoneCols" class="name-path">{{ data.path }}</span>
                    </template>
                </Column>
                <Column field="path" header="Path" :hidden="phoneCols" />
                <Column header="Excludes" :hidden="phoneCols" style="width: 7rem; text-align: right">
                    <template #body="{ data }">
                        <span v-tooltip.top="excludesTooltip(data)" data-test="scan-folder-excludes">
                            {{ data.exclude_patterns.length }}
                        </span>
                    </template>
                </Column>
                <Column header="Symlinks" :hidden="phoneCols" style="width: 9rem">
                    <template #body="{ data }">
                        {{ data.follow_symlinks ? 'Followed' : 'Not followed' }}
                    </template>
                </Column>
                <!-- Narrower on a phone: Name has to keep enough room for the
                     wrapped path it carries there, and Status for the reason. -->
                <Column
                    field="track_count"
                    header="Tracks"
                    :style="
                        phoneCols
                            ? 'width: 4rem; text-align: right'
                            : 'width: 7rem; text-align: right'
                    "
                />
                <Column header="Status" :style="phoneCols ? 'width: 8rem' : 'width: 10rem'">
                    <template #body="{ data }">
                        <Tag
                            :severity="data.available ? 'success' : 'danger'"
                            :value="data.available ? 'Available' : 'Not usable'"
                            v-tooltip.top="data.available ? undefined : data.problem"
                            data-test="scan-folder-status"
                        />
                        <!-- The tooltip alone is unreachable by touch and by
                             keyboard, and this read-only panel has no row
                             dialog to read the reason in instead. -->
                        <div
                            v-if="!data.available && data.problem"
                            class="status-problem"
                            data-test="scan-folder-problem"
                        >
                            {{ data.problem }}
                        </div>
                    </template>
                </Column>
            </DataTable>
        </div>
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
    margin-bottom: 0.5rem;
}
.section h2 {
    font-size: 1.25rem;
    font-weight: 600;
    margin: 0;
}
.hint {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
    margin: 0 0 1rem;
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
.error-state {
    text-align: center;
    padding: 2rem;
    color: var(--app-danger);
}
.folder-name {
    display: inline-flex;
    align-items: center;
}
/* The reason quotes a filesystem path, and a long path is one unbreakable
   token: left to itself it widens the Status column rather than wrapping.
   Deliberately NOT limited to the phone tier: it adds no width on desktop
   either, it only lets an over-long reason break. */
.status-problem {
    margin-top: 0.25rem;
    font-size: 0.8rem;
    color: var(--app-text-secondary);
    overflow-wrap: anywhere;
}
/* The path wraps instead of being clipped: a table cell with no width
   constraint grows to fit nowrap content, so an ellipsis never triggers —
   the cell simply widened the whole table until the Status column (and an
   unusable folder's reason with it) sat off the side of a phone screen. */
.name-path {
    display: block;
    margin-top: 0.25rem;
    font-size: 0.8rem;
    color: var(--app-text-secondary);
    white-space: normal;
    overflow-wrap: anywhere;
}
.table-fit {
    overflow-x: auto;
}
</style>
