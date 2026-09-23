<script setup lang="ts">
import LibrariesPanel from '@/components/admin/LibrariesPanel.vue'
import MainLibraryPanel from '@/components/admin/MainLibraryPanel.vue'
import ScanFoldersPanel from '@/components/admin/ScanFoldersPanel.vue'
</script>

<template>
    <div class="libraries-view">
        <ScanFoldersPanel />
        <hr class="section-divider" />
        <MainLibraryPanel />
        <hr class="section-divider" />
        <LibrariesPanel />
    </div>
</template>

<style scoped>
.libraries-view {
    padding: 2rem;
    overflow-y: auto;
}
/* The panel above already ends in a 2.5rem margin; matching it below keeps
   the rule centred between the two sections. */
.section-divider {
    border: none;
    border-top: 1px solid var(--app-border);
    margin: 0 0 2.5rem;
}
</style>
