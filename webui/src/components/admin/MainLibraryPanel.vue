<script setup lang="ts">
import { computed } from 'vue'
import Checkbox from 'primevue/checkbox'
import SidebarLayoutPicker from '@/components/admin/SidebarLayoutPicker.vue'
import { useCatalogSettings, useUpdateCatalogSettings } from '@/composables/useLibraries'

// The root library — the whole catalog, browsed without a library — has two
// settings of its own: how the sidebar lists it, and whether it offers the
// Groupings view. Either saves straight away.
const { data: settings, isLoading, isError } = useCatalogSettings()
const update = useUpdateCatalogSettings()

const splitViews = computed<boolean>({
    get: () => settings.value?.split_views ?? true,
    set: (split) => {
        if (split === settings.value?.split_views) return
        update.mutate({ split_views: split })
    }
})

const showGroupings = computed<boolean>({
    get: () => settings.value?.show_groupings ?? false,
    set: (show) => {
        if (show === (settings.value?.show_groupings ?? false)) return
        update.mutate({ show_groupings: show })
    }
})
</script>

<template>
    <section class="section">
        <div class="section-header">
            <h2 id="main-library-heading">Main library</h2>
        </div>
        <p class="hint">
            The whole catalog, above the libraries in the sidebar: its Discover, Artists and Albums
            as an entry each, or one entry that switches between them.
        </p>

        <div v-if="isLoading" class="loading">
            <i class="icon-spin ms-progress-activity" style="font-size: 1.5rem"></i>
        </div>
        <div v-else-if="isError" class="error-state" data-test="main-library-error">
            Could not load the main library's settings. Check that the server is reachable and
            reload the page.
        </div>
        <SidebarLayoutPicker
            v-else
            v-model="splitViews"
            name="main-library-sidebar"
            ariaLabelledby="main-library-heading"
            :class="{ saving: update.isPending.value }"
        />
        <div v-if="!isLoading && !isError" class="check-option" :class="{ saving: update.isPending.value }">
            <Checkbox
                v-model="showGroupings"
                binary
                inputId="main-library-groupings"
                data-test="main-library-groupings"
            />
            <label for="main-library-groupings">
                Show Collections view
                <span class="check-hint">
                    Albums grouped by their Grouping tag, like a franchise or a series.
                </span>
            </label>
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
.error-state {
    text-align: center;
    padding: 2rem;
    color: var(--app-danger);
}
.check-option {
    display: flex;
    align-items: flex-start;
    gap: 0.6rem;
    margin-top: 1.25rem;
}
.check-hint {
    display: block;
    color: var(--app-text-secondary);
    font-size: 0.85rem;
}
.saving {
    opacity: 0.6;
    pointer-events: none;
}
</style>
