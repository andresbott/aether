<script setup lang="ts">
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
</script>

<template>
    <ContentScaffold title="Genres">
        <div class="genres-scroll">
            <div class="empty-state">
                <i class="pi pi-tags" style="font-size: 3rem"></i>
                <p>Genres view coming soon</p>
            </div>
        </div>
    </ContentScaffold>
</template>

<style scoped>
.genres-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
}
.empty-state { display: flex; flex-direction: column; align-items: center; padding: 4rem; gap: 1rem; color: var(--app-text-secondary); }
</style>
