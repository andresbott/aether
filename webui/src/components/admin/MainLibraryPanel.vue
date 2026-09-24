<script setup lang="ts">
import { computed } from 'vue'
import SidebarLayoutPicker from '@/components/admin/SidebarLayoutPicker.vue'
import { useCatalogSettings, useUpdateCatalogSettings } from '@/composables/useLibraries'

// The root library — the whole catalog, browsed without a library — has one
// setting of its own: how the sidebar lists it. Picking a card saves it.
const { data: settings, isLoading, isError } = useCatalogSettings()
const update = useUpdateCatalogSettings()

const splitViews = computed<boolean>({
    get: () => settings.value?.split_views ?? true,
    set: (split) => {
        if (split === settings.value?.split_views) return
        update.mutate({ split_views: split })
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
.saving {
    opacity: 0.6;
    pointer-events: none;
}
</style>
