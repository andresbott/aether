<script setup lang="ts">
import LibrariesPanel from '@/components/admin/LibrariesPanel.vue'
import ScanFoldersPanel from '@/components/admin/ScanFoldersPanel.vue'
</script>

<template>
    <div class="libraries-view">
        <LibrariesPanel />
        <ScanFoldersPanel />
    </div>
</template>

<style scoped>
.libraries-view {
    padding: 2rem;
    overflow-y: auto;
}
</style>
