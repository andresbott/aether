<script setup lang="ts">
</script>

<template>
    <div class="genres-view">
        <div class="view-header">
            <h1>Genres</h1>
        </div>

        <div class="empty-state">
            <i class="pi pi-tags" style="font-size: 3rem"></i>
            <p>Genres view coming soon</p>
        </div>
    </div>
</template>

<style scoped>
.genres-view { max-width: 1400px; margin: 0 auto; }
.view-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 2rem; }
.view-header h1 { font-size: 2rem; font-weight: 700; margin: 0; }
.empty-state { display: flex; flex-direction: column; align-items: center; padding: 4rem; gap: 1rem; color: var(--app-text-secondary); }
</style>
